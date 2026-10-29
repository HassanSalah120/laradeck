import React from "react";
import { createRoot } from "react-dom/client";
import "./workbench.css";
import App from "./Workbench";
import { initializePlatform } from "./browserPlatform";

const container = document.getElementById("root");

const root = createRoot(container!);

initializePlatform().then(() => root.render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
)).catch(error => { if (container) container.textContent = error instanceof Error ? error.message : String(error); });
