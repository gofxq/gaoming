import React from "react";
import ReactDOM from "react-dom/client";
import { App } from "./app/App";

import "./styles/global.css";

const savedTheme = window.localStorage.getItem("gaoming-theme");
const initialTheme =
  savedTheme === "light" || savedTheme === "dark"
    ? savedTheme
    : window.matchMedia("(prefers-color-scheme: dark)").matches
      ? "dark"
      : "light";

document.documentElement.dataset.theme = initialTheme;
if (initialTheme === "dark") {
  document.body.setAttribute("theme-mode", "dark");
}

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
);
