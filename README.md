# Simple avatar service meant for intranet usage

There are public services, like [Gravatar](http://www.gravatar.com), where one can register an Avatar, which can then be displayed
on for example forums. There is also a growing number of tools that are used inside companies that can use such a
global avatar service. However, not all people may want to register at a global service.

The solution is to setup an avatar service on the intranet, for example using [libravatar](https://www.libravatar.org/). While this
is not too difficult to setup, it still requires some effort.

Intravatar's goal is to be as simple as possible to setup. Avatars can be uploaded by users and/or maintained by an
administrator. Besides that Intravatar can be configured to use a remote service to fallback on when an image is not
found in the local database.

Usage:

There are no binaries available yet. To install and run, [Golang](https://golang.org/dl/) is required. After installing Golang run
the following from the command prompt to install and run:

```bash
go get -u -a github.com/bertbaron/intravatar
cd $GOCODE/src/github.com/bertbaron/intravatar
go build
./intravatar
```

Or on Windows (totally untested)
```bat
go get -u -a github.com/bertbaron/intravatar
cd %GOPATH%\src\github.com\bertbaron\intravatar
go build
intravatar
```

After this, point your webbrowser to [http://localhost:8080/upload/form](http://localhost:8080/upload/form).

Run `./intravatar -h` to get a list of command-line options.
