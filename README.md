# Simple avatar service meant for intranet usage

[![](https://img.shields.io/microbadger/image-size/bertbaron/intravatar.svg)](http://microbadger.com/images/bertbaron/intravatar)
[![](https://img.shields.io/microbadger/layers/bertbaron/intravatar.svg)](http://microbadger.com/images/bertbaron/intravatar)
[![](https://img.shields.io/github/release/bertbaron/intravatar.svg)](https://github.com/bertbaron/intravatar/releases/latest)
[![](https://img.shields.io/travis/bertbaron/intravatar.svg?branch=master)](https://travis-ci.org/bertbaron/intravatar)

There are public services, like [Gravatar](http://www.gravatar.com), where one can register an Avatar, which can then be displayed on for example forums. There is also a growing number of tools that are used inside companies that can use such a global avatar service. However, not all people may want to register at a global service or company policy may not allow it.

The solution is to setup an avatar service on the intranet, for example using [libravatar](https://www.libravatar.org/). While this is not too difficult to setup, it still requires some effort.

Intravatar's goal is to be as simple as possible to setup. Avatars can be uploaded by users and/or maintained by an administrator. Besides, Intravatar can be configured as a proxy, using a remote service as fallback for missing images.

## Installation (without docker)

Download the latest release

[![](https://img.shields.io/github/release/bertbaron/intravatar.svg)](https://github.com/bertbaron/intravatar/releases/latest)

Unpack and run the `intravatar` or `intravatar.exe` executable in the intravatar directory.
Adjust `config.ini` where necessary (changes will have effect after a restart).

## How to use the docker image

### Run with minimal configuration
```
  docker run -p 8080:8080 -v /var/lib/intravatar:/intravatar/data bertbaron/intravatar -h $(hostname)
```

 * This will provide the service at <http://localhost:8080>.
 * Data will be stored under /var/lib/intravatar.
 * Users can upload avatars without confirmation email (configure smtp for email confirmation)
 * When no image is found gravatar is used as fallback, finally falling back on a generated monster id.

### Show usage information:

```shell
docker run --rm bertbaron/intravatar -h
```

Note that the current working directory in the container is `/intravatar`.

### Relevant mount points

 * `intravatar/data` - data directory, should be mounted to make the data persistent
 * `intravatar/config.ini` - configuration file, can be mounted to avoid the need for command line options (although those would still take precedence)
 * `intravatar/resources/templates` - html template files, can be customized
 * `intravatar/resources/static` - static files that can be used by the html templates. By default contains robots.txt and stylesheet.css.

Refer to <https://github.com/bertbaron/intravatar> for the default files (or download and unpack a released version from <https://github.com/bertbaron/intravatar/releases>)

## Feedback

Please let me know via github or docker hub if you find an issue or would like to suggest a feature to be added.