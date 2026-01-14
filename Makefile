build:
	go build -o gitty main.go

cp:
	cp gitty ~/.local/bin/
	
install: build cp