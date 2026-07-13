import React from "react";
import ReactDOM from "react-dom/client";
import { App } from "./app/App";

import "./styles/global.css";
import "./shared/styles/themes/pixel/index.css";
import "./pc/components/pixel/PcPixel.css";

const savedTheme = window.localStorage.getItem("gaoming-theme");
const savedSkin = window.localStorage.getItem("gaoming-skin");
const initialTheme =
  savedTheme === "light" || savedTheme === "dark"
    ? savedTheme
    : window.matchMedia("(prefers-color-scheme: dark)").matches
      ? "dark"
      : "light";
const initialSkin = savedSkin === "modern" || savedSkin === "pixel" ? savedSkin : "pixel";

document.documentElement.dataset.theme = initialTheme;
document.documentElement.dataset.skin = initialSkin;
if (initialTheme === "dark") {
  document.body.setAttribute("theme-mode", "dark");
}

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
);
