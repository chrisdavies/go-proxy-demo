# Go Proxy Demo

This is an example project based on a proxy I built for work. It uses SQLite to drive proxy rules, certmagic to handle TLS automatically, and systemd sockets to manage zero-downtime deploys.

The intent of the proxy is to allow content to be migrated from a v1 service to a v2 service.

## TLS

TLS is handled automatically by [certmagic](https://pkg.go.dev/github.com/caddyserver/certmagic).

This application accepts *any* domain, which might be a problem. Certmagic can whitelist domains instead.

The line that configures the app to allow any domain is in `serve.go`:

```go
certmagic.Default.OnDemand = new(certmagic.OnDemandConfig)
```

## Systemd sockets

Systemd sockets allow systemd to manage incoming requests. When a request comes in, systemd will attempt to start our application if it is down. It will then hand the connection(s) to our application when our application is ready.

This means, we can deploy a new version of our application, and all we need to do is stop the old one and start the new one. Systemd will ensure the handoff is smooth.

The build scripts produce binaries that look something like this `go-proxy-demo-20221708303201`.

We have a symlink `go-proxy-demo` which points to the latest binary. This is what the `systemd.service` runs.

## Tunneling

The one nuisance to this whole approach was figuring out how to pass systemd sockets to certmagic. Certmagic listens on *ports* but systemd sockets give us *file descriptors*. I wasn't able to figure out how to make this work directly, so... Hack time. I had certmagic listen on ports that I specified. I then created a TCP proxy which received connections on the file descriptors and proxied them to certmagic.

In retrospect, it might have been simpler to just avoid systemd and certmagic and isntead write a basic proxy that sat behind Caddy.

Anyway, all of this hackiness can be seen in `serve.go`.

## Note

I haven't actually tested this project. It's a modified version of a private project which I scrubbed and am making public for instructional purposes.

## Requirements

- Due to SQLite, this requires CGO
- `./run-dev.sh` is a handy dev script, but it has some requirements:
  - [reflex](https://github.com/cespare/reflex) for rebuilding on save
  - systemd (so, basically, a modern Linux distro)

## SQLite errors

When running this through a benchmark, I noticed that SQLite causes errors under heavy load when writes are made concurrently. The solution (in the private project) was to use SQLite as a store only and keep the rules in memory. All SQLite access was managed via a channel as part of the solution.

## License

[MIT](https://mit-license.org/)

