# tinystatic 

![logo.png](https://github.com/julvo/tinystatic/blob/master/logo.png "tinystatic logo")

A tiny static website generator that is flexible and easy to use. It's flexible, as there is no required website structure nor any blog-specific concepts. It's easy to use, as the API is kept minimal and we can start with a standard HTML site and start using tinystatic gradually.

## Install

### Pre-built binaries
Download the tinystatic binary for your operating system:
- [Linux](https://github.com/julvo/tinystatic/releases/download/v0.0.1/tinystatic_linux_amd64) 
- [macOS](https://github.com/julvo/tinystatic/releases/download/v0.0.1/tinystatic_macos_darwin_amd64) 

Optionally, add the binary to your shell path, by either placing the binary into an existing directory like `/usr/bin` or by adding the parent directory of the binary to your path variable.

If you added tinystatic to your path, you should be able to call
```shell
tinystatic -help
```
Otherwise, you will need to specify the path to the tinystatic binary when calling it
```shell
/path/to/tinystatic -help
```

### Compiling yourself
First, you will need to install the Golang compiler to compile tinystatic. Then, you can install tinystatic by running
```shell
go install -u github.com/julvo/tinystatic
```
or by cloning the repository and running `go install` or `go build` in the root directory of this repository.
