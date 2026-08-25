create_bundle:
	npx --prefix ./assets/node_modules rollup assets/js/index.js -f iife -o assets/js/dist/index.bundle.js -p @rollup/plugin-node-resolve --output.name cm6 && \
	npx --prefix ./assets/node_modules  minify assets/js/dist/index.bundle.js > assets/js/dist/index.bundle.min.js

bundle: create_bundle
	echo "Bundle generation finished"

install_templ:
			go install github.com/a-h/templ/cmd/templ@latest

make_templ: install_templ
		templ generate

templ: make_templ create_bundle
	echo "Templates generation finished"
