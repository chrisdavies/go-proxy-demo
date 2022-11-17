# Deploy

## Initial setup

To set the proxy up in a production environment:

- Configure the environment:
  - Copy `./example.env` to `/dat/example/go-proxy-demo`
  - Rename to `.env`
  - Edit to contain the proper values
- Configure systemd:
  - Copy the socket and service files to `/lib/systemd/system/`
- Deploy:
  - Run `./deploy.sh {hostname | ip}`

