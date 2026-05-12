/**
 * Bootstrap. Mounts the Svelte app, installs the theme attribute, starts
 * the data store lifecycle. Kept intentionally tiny: main.ts is the only
 * module that directly touches `document`.
 */

import "./styles/base.css";

import { mount } from "svelte";
import App from "./App.svelte";
import { initTheme } from "./lib/theme";

initTheme();

const target = document.getElementById("app");
if (!target) {
  throw new Error("mansion: #app root element missing from index.html");
}

mount(App, { target });
