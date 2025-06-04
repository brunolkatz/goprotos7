install_bindata:
		go install github.com/go-bindata/go-bindata/go-bindata@latest

create_bindata:
	go-bindata -pkg assets_files_watcher -fs \
	-o ./api/assets-files-watcher-api/assets-files-bindata.go \
	--prefix assets \
	./assets/css/*.css \
	./assets/js/bootstrap.bundle.min.js \
	./assets/js/htmx.min.js \
	./assets/js/color-modes.js \
	./assets/js/dist/*.min.js \
	./assets/images/htmx_with_lasers.png \
	./assets/fonts/*.ttf ./assets/fonts/*.woff ./assets/fonts/*.woff2 ./assets/fonts/*.eot

create_bundle:
	npx --prefix ./assets/node_modules rollup assets/js/index.js -f iife -o assets/js/dist/index.bundle.js -p @rollup/plugin-node-resolve --output.name cm6 && \
	npx --prefix ./assets/node_modules  minify assets/js/dist/index.bundle.js > assets/js/dist/index.bundle.min.js

bundle: create_bundle
	echo "Bundle generation finished"

install_templ:
			go install github.com/a-h/templ/cmd/templ@latest

make_templ: install_templ
		templ generate

templ: make_templ create_bundle create_bindata
	echo "Templates generation finished"
