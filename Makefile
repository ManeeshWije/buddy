all: build

build-lambda: build
	zip -r lambda.zip node_modules/* *.js

build:
	npm run build

run:
	npm run cli

clean:
	rm -f cli.js index.js lambda.zip

deploy: build-lambda
	terraform apply
