/** @type {import("@sveltejs/vite-plugin-svelte").SvelteConfig} */
export default {
  compilerOptions: {
    // Enable store reactivity ($store syntax) in Svelte 5
    compatibility: {
      componentApi: 4,
    },
  },
}
