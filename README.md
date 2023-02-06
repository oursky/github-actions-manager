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

### Receive Webhook on local development server

1. Setup Reverse Proxy by following the [Notion Guide](https://www.notion.so/oursky/Reverse-HTTPS-Proxy-with-Pandawork-49f7b102c1524a5fb3b00bc55a8c4fb6).

2. Go to `Webhook` tab of `Settings` page of a Repository with Action Runner Setup (e.g. https://github.com/oursky/github-actions-manager/settings). Press `Add webhook`.

3. Fill in `Payload URL` with the reverse proxy URL according to the [Notion Guide](https://www.notion.so/oursky/Reverse-HTTPS-Proxy-with-Pandawork-49f7b102c1524a5fb3b00bc55a8c4fb6).

4. Fill in `Secret` with the same value of `webhookSecret` in `config.toml`.

5. Select `Send me everything.` in `Which events would you like to trigger this webhook?`.

6. Save Webhook

### Connect Slack App to local development server

1. On Slack Portal https://api.slack.com/apps, press `Create New App` and select `From scratch`

2. Enable `Slash Commands` and `Bots` on `Add features and functionality`.

3. Enable Socket Mode on `Socket Mode`.

4. Add `/gha` to `Slash Commands`.

5. Add an access token on `App-Level Tokens`, copy the value to `appToken` in `config.toml`.

6. Install the app to workspace on `OAuth & Permissions` and copy `Bot User OAuth Token` to `botToken` in `config.toml`. Also make sure to add scope `chat:write` to `Bot Token Scopes` .

7. Integrate app on the channel
