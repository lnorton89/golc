import React from "react";
import ReactDOM from "react-dom/client";

// WR-02 fix: Archivo/JetBrains Mono are self-hosted (bundled by vite build)
// via @fontsource rather than loaded from fonts.googleapis.com on every
// launch -- a lighting-control desktop app is a plausible candidate for an
// isolated/offline show network, and the rest of this codebase already
// keeps the Art-Net daemon/MIDI/safety paths fully local. Weights mirror
// index.html's former Google Fonts request exactly: Archivo
// 400/500/600/700/800/900, JetBrains Mono 400/500/600.
import "@fontsource/archivo/400.css";
import "@fontsource/archivo/500.css";
import "@fontsource/archivo/600.css";
import "@fontsource/archivo/700.css";
import "@fontsource/archivo/800.css";
import "@fontsource/archivo/900.css";
import "@fontsource/jetbrains-mono/400.css";
import "@fontsource/jetbrains-mono/500.css";
import "@fontsource/jetbrains-mono/600.css";

import App from "./App";
import "./index.css";

ReactDOM.createRoot(document.getElementById("root") as HTMLElement).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
);
