build:
	go build -o gitty
cp:
	cp gitty ~/.local/bin/
	
install: build cp