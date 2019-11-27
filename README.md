# notpecl

`notpecl` is a little CLI tool to replace the old `pecl` tool, as the latter is
not shipped with PHP since 7.4. It has a simple UI, with only three commands:

* `download`: Download and unpack extension archives (tgz with a package.xml) ;
* `build`: Build an extension from its source code ;
* `install`: Download and build an extension locally. This is the one you 
probably want to use ;

Like pecl, this tool supports config questions and can be run in noninteractive
mode (it uses default values in such case).

There's no support for version constraints for now, but it's going to be
provided at some point.

It's only compatible with Unix systems for now, but support for Windows is 
planned.

## Install

For now, you have to build it by yourself but a proper release will come:

```
go get github.com/NiR-/notpecl
go install github.com/NiR-/notpecl
```
