// setup.ts wires @testing-library/jest-dom's matchers into every Vitest
// file (toBeInTheDocument, etc.) -- referenced by vite.config.ts's
// test.setupFiles.
import "@testing-library/jest-dom/vitest";
