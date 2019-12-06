# notpecl

`notpecl` is a little CLI tool to replace the old `pecl` tool, as the latter is
not shipped with PHP since 7.4. It has a simple UI, with only three commands:

* `download`: Download and unpack extension archives (tgz with a package.xml) ;
* `build`: Build an extension from its source code ;
* `install`: Download and build an extension locally. This is the one you 
probably want to use ;

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
chmod +x /usr/ocal/sbin/notpecl
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
