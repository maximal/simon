all: build

build:
	#
	# Building project...
	#
	go build -ldflags '-s -w' -trimpath
	#
	# Done.
	#

build_vsc:
	#
	# Building project...
	#
	go build -ldflags '-s -w' -trimpath -buildvcs
	#
	# Done.
	#

update:
	#
	# Updating GO modules...
	#
	go get -u ./...
	#
	# Tidying GO modules...
	#
	go mod tidy

format:
	#
	# Formatting GO files...
	#
	find . -type f -name '*.go' -exec go fmt '{}' \;
