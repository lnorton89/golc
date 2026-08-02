// FixtureStyleModal is the pencil-icon edit dialog for one fixture card's
// own visual customization (background color, font color, background
// image + fit/fill) -- reuses ConfirmModal's own backdrop+dialog shell
// pattern (backdrop click + Escape both cancel, no Radix dependency)
// rather than a new one, and Field for each labeled input, mirroring
// ScriptRunDialog's identical "form inside the ConfirmModal shell"
// composition.
//
// The background image itself is a native-file-picker upload, not a URL
// field: pickImageFile opens App.PickImageFile's own OS dialog (filtered
// to common raster/vector formats, animated GIF included), and
// uploadImage reads + persists the chosen file's bytes as a new asset row
// inside the show's own .golc SQLite file (internal/show/assets.go's own
// doc comment covers why a whole new table exists for this). Only the
// resulting asset id is ever stored in FixtureStyle -- the actual bytes
// live exactly once, in the show file, never duplicated into localStorage
// alongside the rest of this feature's own per-card customization.
import { useEffect, useState } from "react";
import { RotateCcw } from "lucide-react";

import { deleteImage, pickImageFile, uploadImage } from "../../lib/wailsBridge";
import Button from "../primitives/Button/Button";
import Field from "../primitives/Field/Field";
import styles from "./FixtureStyleModal.module.css";

/** BackgroundSize is FixtureStyle's own fit/fill setting, one-to-one with
 * a CSS background-size value -- "stretch" maps to the literal "100%
 * 100%" keyword pair (background-size has no single "stretch" keyword of
 * its own), everything else is the CSS keyword unchanged. */
export type BackgroundSize = "cover" | "contain" | "stretch";

export const BACKGROUND_SIZE_CSS_VALUE: Record<BackgroundSize, string> = {
  cover: "cover",
  contain: "contain",
  stretch: "100% 100%",
};

export interface FixtureStyle {
  backgroundColor?: string;
  fontColor?: string;
  /** backgroundImageAssetID references a row in the show's own assets
   * table (never a URL, never the image's own bytes) -- Desk.tsx resolves
   * it to a data: URI once per session via getImageDataURI, cached so
   * every card sharing the same asset only ever fetches it once. */
  backgroundImageAssetID?: string;
  /** backgroundSize only matters when backgroundImageAssetID is set --
   * defaults to "cover" (fixtureCardInlineStyle's own fallback) when
   * unset, matching this modal's own default selection for a fresh
   * upload. */
  backgroundSize?: BackgroundSize;
}

interface FixtureStyleModalProps {
  fixtureName: string;
  initialStyle: FixtureStyle;
  /** initialImageDataURI is the CURRENT backgroundImageAssetID's already-
   * resolved preview, handed down from Desk.tsx's own asset cache so this
   * modal never needs its own separate fetch just to show what's already
   * saved -- undefined when initialStyle has no image, or the very first
   * render before Desk's cache has resolved it yet (rare: cards resolve
   * eagerly on mount, well before an operator reaches for the pencil
   * icon). */
  initialImageDataURI?: string;
  /** onSave's second argument is the CURRENT preview's own data: URI
   * (undefined when no image is set) -- lets Desk.tsx seed its own
   * imageDataUriCache directly from a fresh upload instead of re-fetching
   * an asset it already has the bytes for. */
  onSave: (style: FixtureStyle, previewDataURI: string | undefined) => void;
  onClose: () => void;
}

/** readThemeColorHex reads a CSS custom property's own current value
 * straight off the document (index.css's own tokens are already plain hex
 * strings, e.g. "--page: #e4e0d8", never resolved through color-mix() or
 * similar) -- used only to give the color pickers below a sensible
 * starting point that matches whichever theme is active, never persisted
 * itself unless the operator actually changes it. */
function readThemeColorHex(varName: string, fallback: string): string {
  if (typeof window === "undefined") return fallback;
  const value = getComputedStyle(document.documentElement).getPropertyValue(varName).trim();
  return value || fallback;
}

export default function FixtureStyleModal({
  fixtureName,
  initialStyle,
  initialImageDataURI,
  onSave,
  onClose,
}: FixtureStyleModalProps) {
  const [backgroundColor, setBackgroundColor] = useState(initialStyle.backgroundColor);
  const [fontColor, setFontColor] = useState(initialStyle.fontColor);
  const [backgroundImageAssetID, setBackgroundImageAssetID] = useState(initialStyle.backgroundImageAssetID);
  const [backgroundSize, setBackgroundSize] = useState<BackgroundSize>(initialStyle.backgroundSize ?? "cover");
  const [previewDataURI, setPreviewDataURI] = useState(initialImageDataURI);
  const [uploading, setUploading] = useState(false);
  const [uploadError, setUploadError] = useState<string | null>(null);

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") onClose();
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [onClose]);

  const handleChooseImage = async () => {
    setUploadError(null);
    const path = await pickImageFile();
    if (!path) return; // cancelled -- not an error
    setUploading(true);
    const result = await uploadImage(path);
    setUploading(false);
    if (!result) {
      setUploadError("Couldn't upload that image -- it may be too large or not a recognized image format.");
      return;
    }
    setBackgroundImageAssetID(result.id);
    setPreviewDataURI(result.dataUri);
  };

  const handleClearImage = () => {
    setUploadError(null);
    setBackgroundImageAssetID(undefined);
    setPreviewDataURI(undefined);
  };

  const handleSave = () => {
    // A replaced or cleared image's OLD asset is deleted only now (never
    // eagerly on the Clear/re-pick click itself) -- Cancel after Clear
    // must leave the original asset untouched, and there's no reason to
    // keep an orphaned blob around once Save actually commits away from
    // it. Fire-and-forget: this card's own save doesn't need to wait on
    // cleanup of an asset nothing references anymore.
    const oldAssetID = initialStyle.backgroundImageAssetID;
    if (oldAssetID && oldAssetID !== backgroundImageAssetID) {
      void deleteImage(oldAssetID);
    }
    onSave(
      {
        backgroundColor,
        fontColor,
        backgroundImageAssetID,
        backgroundSize: backgroundImageAssetID ? backgroundSize : undefined,
      },
      backgroundImageAssetID ? previewDataURI : undefined,
    );
  };

  return (
    <div className={styles.backdrop} onClick={onClose}>
      <div
        className={styles.dialog}
        role="dialog"
        aria-modal="true"
        aria-label={`Customize ${fixtureName}`}
        onClick={(event) => event.stopPropagation()}
      >
        <h3 className={styles.title}>Customize card</h3>
        <p className={styles.subtitle} title={fixtureName}>
          {fixtureName}
        </p>

        <div className={styles.fieldRow}>
          <Field
            label="Background color"
            type="color"
            value={backgroundColor ?? readThemeColorHex("--page", "#e4e0d8")}
            onChange={(event) => setBackgroundColor(event.target.value)}
          />
          <button
            type="button"
            className={styles.resetButton}
            aria-disabled={backgroundColor === undefined}
            onClick={backgroundColor === undefined ? undefined : () => setBackgroundColor(undefined)}
            title={backgroundColor === undefined ? "Using the default background color" : "Reset to the default background color"}
          >
            <RotateCcw size={13} aria-hidden="true" />
          </button>
        </div>

        <div className={styles.fieldRow}>
          <Field
            label="Font color"
            type="color"
            value={fontColor ?? readThemeColorHex("--ink", "#2b2a25")}
            onChange={(event) => setFontColor(event.target.value)}
          />
          <button
            type="button"
            className={styles.resetButton}
            aria-disabled={fontColor === undefined}
            onClick={fontColor === undefined ? undefined : () => setFontColor(undefined)}
            title={fontColor === undefined ? "Using the default font color" : "Reset to the default font color"}
          >
            <RotateCcw size={13} aria-hidden="true" />
          </button>
        </div>

        <div className={styles.imageSection}>
          <span className={styles.imageSectionLabel}>Background image</span>
          <div className={styles.imagePreviewRow}>
            {previewDataURI ? (
              <img className={styles.imagePreview} src={previewDataURI} alt="" />
            ) : (
              <div className={styles.imagePreviewEmpty}>No image</div>
            )}
            <div className={styles.imageButtons}>
              <Button variant="secondary" onClick={() => void handleChooseImage()} disabled={uploading}>
                {uploading ? "Uploading…" : "Choose Image…"}
              </Button>
              <button
                type="button"
                className={styles.resetButton}
                aria-disabled={backgroundImageAssetID === undefined}
                onClick={backgroundImageAssetID === undefined ? undefined : handleClearImage}
                title={backgroundImageAssetID === undefined ? "No background image set" : "Clear the background image"}
              >
                <RotateCcw size={13} aria-hidden="true" />
              </button>
            </div>
          </div>
          {uploadError && <p className={styles.uploadError}>{uploadError}</p>}
        </div>

        {backgroundImageAssetID && (
          <Field label="Image fit">
            <select
              className={styles.select}
              value={backgroundSize}
              onChange={(event) => setBackgroundSize(event.target.value as BackgroundSize)}
            >
              <option value="cover">Fill (crop to fit the card)</option>
              <option value="contain">Fit (show the whole image)</option>
              <option value="stretch">Stretch (ignore aspect ratio)</option>
            </select>
          </Field>
        )}

        <div className={styles.actions}>
          <Button variant="secondary" autoFocus onClick={onClose}>
            Cancel
          </Button>
          <Button variant="primary" onClick={handleSave}>
            Save
          </Button>
        </div>
      </div>
    </div>
  );
}
