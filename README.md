# Github Actions Manager

This app has three components: the GitHub Action repo, the intermediate server, and the Slack interface.
It is recommended to create your own repo to test the app, using the 2,000 free hours they give you.

### Running local development server

On root directory:

```bash
cp examples/config.dev.toml config.toml 
```

Follow the directons below to generate values of `token`, `botToken` and `appToken` for `config.toml`. Then:

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

2. Under `Basic Information` -> `App-Level Tokens`, add an access token with scope `[connections:write]`. Copy the generated `Token` to `appToken` in `config.toml`.

3. Under `Socket Mode`, enable Socket Mode.

4. Under `Add features and functionality` -> `Slash Commands` -> `Create New Command`, add a command. (It is advidable to pick a command prefix that does not overlap with existing bot commands.) In `config.toml`, change `commandName` to your chosen prefix. 

5. Under `OAuth & Permissions` -> `Scopes`, select `commands` and `chat:write.public` (which will autoselect `chat:write`).

6. Under `OAuth & Permissions` -> `OAuth Tokens`, install the app to the workspace. Copy the generated `Bot User OAuth Token` to `botToken` in `config.toml`.

7. Under Github tokens page https://github.com/settings/tokens, generate a Github personal access token (classic) with scope `[workflow, notifications]` (this may be more than strictly necessary). Copy the generated `token` to `token` in `config.toml`.

8. Test the app **in a public channel** (e.g. #team-bot-sandbox). The app was not designed with direct messages in mind and may not work there.