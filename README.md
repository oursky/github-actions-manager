# Github Actions Manager

### Running local development server

On root directory:

```bash
cp examples/config.dev.toml config.toml 
```

Replace values of `token`, `botToken` and `appToken` in `config.toml`. Then:

```golang
go run ./cmd/github-actions-manager -config config.toml -loglevel DEBUG
```
