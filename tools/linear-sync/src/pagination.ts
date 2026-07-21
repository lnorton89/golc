// pagination.ts exhausts every page of one Linear Relay-style GraphQL
// connection before any caller may treat the read as complete (CONTEXT
// D-21; T-01-39). A hidden later page could otherwise spoof absence or
// uniqueness during identity discovery (Go, internal/trace/reconcile).
// fetchAllPages tracks the exact cursor history of one connection walk and
// fails closed -- it never loops forever and never silently stops early --
// the moment a cursor repeats, arrives null/empty while more pages are
// claimed to exist, loops back to an earlier-seen cursor, or the walk
// exceeds a defensive page ceiling. This module makes no GraphQL/SDK call
// itself: fetchPage is supplied by the caller (adapter.ts for a real
// @linear/sdk connection, or a fixture-driven fake in tests), so pagination
// exhaustion is provable entirely offline.

/**
 * PageInfo is the exact Relay pagination cursor shape every Linear SDK
 * connection result exposes: hasNextPage and endCursor.
 */
export interface PageInfo {
  hasNextPage: boolean;
  endCursor: string | null;
}

/**
 * ConnectionPage is one fetched page of TNode records plus its cursor
 * state for the next page (or the end of the connection).
 */
export interface ConnectionPage<TNode> {
  nodes: TNode[];
  pageInfo: PageInfo;
}

/**
 * FetchPage requests exactly one page of a connection, starting after
 * cursor (null requests the first page).
 */
export type FetchPage<TNode> = (cursor: string | null) => Promise<ConnectionPage<TNode>>;

/**
 * PaginationComplete is fetchAllPages's success outcome: every page was
 * fetched, cursors behaved monotonically throughout, and nodes holds every
 * record in request order across all pages.
 */
export interface PaginationComplete<TNode> {
  complete: true;
  nodes: TNode[];
  pageCount: number;
}

/**
 * PaginationCode enumerates the exact reasons fetchAllPages fails closed.
 * A repeated, null, or indirectly looped cursor are all cursor anomalies
 * (T-01-39); exceeding the defensive page ceiling is a distinct,
 * deliberately conservative safety stop rather than an unbounded loop.
 */
export type PaginationCode =
  | "LINEAR_PAGINATION_CURSOR_ANOMALY_NULL"
  | "LINEAR_PAGINATION_CURSOR_ANOMALY_REPEATED"
  | "LINEAR_PAGINATION_CURSOR_ANOMALY_LOOP"
  | "LINEAR_PAGINATION_MAX_PAGES_EXCEEDED";

/**
 * PaginationIncomplete is fetchAllPages's fail-closed outcome: the walk
 * stopped before genuinely exhausting the connection. nodesSoFar is
 * diagnostic only -- CONTEXT D-21/T-01-39 require every caller to never
 * treat this partial count, or any node collected before the anomaly, as
 * usable for an identity or create/preview decision.
 */
export interface PaginationIncomplete {
  complete: false;
  code: PaginationCode;
  reason: string;
  pageCount: number;
  nodesSoFar: number;
}

export type PaginationResult<TNode> = PaginationComplete<TNode> | PaginationIncomplete;

/**
 * DEFAULT_MAX_PAGES bounds a single connection walk (50,000 objects at the
 * default 50-per-page Linear connection size) -- generous for any real
 * GOLC delivery hierarchy, and a deliberate, named safety stop rather than
 * an unbounded loop if a hostile or buggy transport never reports
 * hasNextPage=false.
 */
export const DEFAULT_MAX_PAGES = 1000;

/**
 * fetchAllPages walks fetchPage from the first page (cursor=null) until it
 * reports hasNextPage=false, returning every node observed across every
 * page. It fails closed -- returning complete=false rather than throwing
 * or looping forever -- the instant a returned cursor is null/empty while
 * more pages are claimed, repeats the cursor just requested, or matches
 * any cursor already seen earlier in this same walk.
 */
export async function fetchAllPages<TNode>(
  fetchPage: FetchPage<TNode>,
  maxPages: number = DEFAULT_MAX_PAGES,
): Promise<PaginationResult<TNode>> {
  const nodes: TNode[] = [];
  const seenCursors = new Set<string>();
  let cursor: string | null = null;
  let pageCount = 0;

  for (;;) {
    if (pageCount >= maxPages) {
      return {
        complete: false,
        code: "LINEAR_PAGINATION_MAX_PAGES_EXCEEDED",
        reason: `exceeded ${maxPages} pages without reaching the end of the connection`,
        pageCount,
        nodesSoFar: nodes.length,
      };
    }

    const page = await fetchPage(cursor);
    pageCount += 1;
    nodes.push(...page.nodes);

    if (!page.pageInfo.hasNextPage) {
      return { complete: true, nodes, pageCount };
    }

    const nextCursor = page.pageInfo.endCursor;
    if (nextCursor === null || nextCursor === "") {
      return {
        complete: false,
        code: "LINEAR_PAGINATION_CURSOR_ANOMALY_NULL",
        reason: "page reported hasNextPage=true but returned a null/empty endCursor",
        pageCount,
        nodesSoFar: nodes.length,
      };
    }
    if (nextCursor === cursor) {
      return {
        complete: false,
        code: "LINEAR_PAGINATION_CURSOR_ANOMALY_REPEATED",
        reason: `endCursor ${JSON.stringify(nextCursor)} repeated the cursor just requested`,
        pageCount,
        nodesSoFar: nodes.length,
      };
    }
    if (seenCursors.has(nextCursor)) {
      return {
        complete: false,
        code: "LINEAR_PAGINATION_CURSOR_ANOMALY_LOOP",
        reason: `endCursor ${JSON.stringify(nextCursor)} was already seen earlier in this walk`,
        pageCount,
        nodesSoFar: nodes.length,
      };
    }
    seenCursors.add(nextCursor);
    cursor = nextCursor;
  }
}
