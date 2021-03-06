# notpecl

`notpecl` is a little CLI tool to replace the deprecated `pecl` tool.

* [Why?](#why)
* [Usage](#usage)
* [Install](#install)
* [Credits](#credits)

## Why?

The version versions of the official Docker image for PHP 7.4 did not include 
`pecl` as this tool has been deprecated. However, as there were no working 
replacement at this time, Docker maintainers decided to include `pecl` in 7.4
images. See [this issue](https://github.com/docker-library/php/issues/846).

Moreover, as I needed an easy way to resolve version constraints on PHP
community extensions from a Go tool, [zbuild](https://github.com/NiR-/zbuild),
the best seemed to build my own tool.

## Usage

* `install`: Download and build an extension locally. This is the one you 
probably want to use ;
* `download`: Download and unpack extension archives (tgz with a package.xml) ;
* `build`: Build an extension from its source code ;

Like pecl, this tool has an interactive UI for config questions and also
supports running in noninteractive mode.

This tool supports version constraints expressed like Composer:

```
# This is going to install the last v5.1 patch version.
$ notpecl install redis:~5.1.0
```

Moreover, notpecl only resolve versions with `stable` stability by default.
You can override that by appending ̀`@<minimum-stability>` to any version
constraint:

```
# This is going to install yaml v2.0.0RC8
$ notpecl install yaml:2.0.0RC8@beta
# This is going to install the last patch version the v0.2 branch of uv extension
$ notpecl install uv:~0.2.0@beta
```

For more details about version constraints, see the [versions](https://getcomposer.org/doc/articles/versions.md)
page from Composer documentation.

For reference, here's the complete list of stability supported by notpecl, in
descending order:

* `stable`
* `beta`
* `alpha`
* `devel`
* `snapshot`

## Install

You can either download notpecl or compile it by yourself:

###### 1a. Download it

```bash
wget -O /usr/local/sbin/notpecl https://storage.googleapis.com/notpecl/notpecl
chmod +x /usr/local/sbin/notpecl
```

###### 1b. Within a Dockerfile

```dockerfile
FROM php:7.4-fpm-buster

RUN curl -f -o /usr/local/sbin/notpecl https://storage.googleapis.com/notpecl/notpecl && \
    chmod +x /usr/local/sbin/notpecl && \
    notpecl install redis:5.1.1 && \
    docker-php-ext-enable redis && \
    rm -rf /usr/local/sbin/notpecl
```

###### 2. Build from sources

You have to build it by yourself but a proper release will come soon:

```bash
go get github.com/NiR-/notpecl
go install github.com/NiR-/notpecl
```

## Credits

Original idea by [Albin Kerouanton](https://github.com/NiR-), supported by
[KNPLabs](https://www.knplabs.com/)
