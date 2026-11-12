import adapter from '@sveltejs/adapter-auto';
import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

export default defineConfig({
	plugins: [
		sveltekit({
			compilerOptions: {
				runes: ({ filename }) =>
					filename.split(/[/\\]/).includes('node_modules') ? undefined : true
			},
			adapter: adapter()
		})
	],
	resolve: {
		alias: {
			'$lib': path.resolve(__dirname, 'src/lib'),
			'#lib': path.resolve(__dirname, 'src/lib')
		}
	},
	server: {
		port: 3000,
		strictPort: true
	},
	preview: {
		port: 3000,
		strictPort: true
	}
});
