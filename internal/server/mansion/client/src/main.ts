/**
 * Bootstrap. Mounts the Svelte app, installs the theme attribute, starts
 * the data store lifecycle. Kept intentionally tiny: main.ts is the only
 * module that directly touches `document`.
 *
 * The first-run Wizard is mounted into its own root (#wizard-root) so it
 * can show without depending on App.svelte. This keeps the two surfaces
 * loosely coupled — App owns the dashboard layout, Wizard owns the
 * onboarding overlay.
 */

import "./styles/base.css";

import { mount } from "svelte";
import App from "./App.svelte";
import Wizard from "./components/Wizard.svelte";
import { initTheme } from "./lib/theme";

initTheme();

const target = document.getElementById("app");
if (!target) {
  throw new Error("mansion: #app root element missing from index.html");
}

mount(App, { target });

const wizardTarget = document.getElementById("wizard-root");
if (wizardTarget) {
  mount(Wizard, { target: wizardTarget });
}
