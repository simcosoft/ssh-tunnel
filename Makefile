.SILENT:

BINARY  = ssh_tunnel
LDFLAGS = -ldflags '-s -w'

ifeq ($(OS),Windows_NT)
    EXT         = .exe
    INSTALL_DIR = C:/Windows/System32
    RM          = del /f /q
    CP          = copy /y
    SET_GOOS    = set GOOS=windows&& set GOARCH=amd64&&
else
    EXT         =
    INSTALL_DIR = /usr/local/bin
    RM          = rm -f
    CP          = cp
    SET_GOOS    = GOOS=windows GOARCH=amd64
endif

TARGET = $(BINARY)$(EXT)

.PHONY: build install build-windows clean test

build:
	go build $(LDFLAGS) -o $(TARGET)

install: build
	$(RM) $(INSTALL_DIR)/$(TARGET)
	$(CP) $(TARGET) $(INSTALL_DIR)

build-windows:
	$(SET_GOOS) go build $(LDFLAGS) -o $(BINARY).exe

clean:
	$(RM) $(BINARY) $(BINARY).exe

test:
	go test ./...
