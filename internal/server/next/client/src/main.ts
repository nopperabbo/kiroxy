/**
 * Bootstrap. Mounts the Svelte app, attaches global hotkeys, starts the SSE
 * connection.
 */

import "./styles/base.css";

import { mount } from "svelte";
import App from "./App.svelte";

const target = document.getElementById("app");
if (!target) {
  // Defensive: should never happen because index.html owns this element, but
  // failing loudly beats a silent blank page if an operator adjusts the HTML.
  throw new Error("Dashboard Next: #app root element missing from index.html");
}

mount(App, { target });
