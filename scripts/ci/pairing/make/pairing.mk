.PHONY: build-pairing
build-pairing:
	# Clean the directory if it exists
	if [ -d "${PAIRING_DIR}" ]; then rm -rf ${PAIRING_DIR}; fi
	# Create the directory
	mkdir -p ${PAIRING_DIR}
	# Build the Go binary into the specified directory
	go build -o ${PAIRING_DIR}/pairing ./cmd