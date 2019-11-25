# Matrix-EmailBridge
A matrix-bridge written in Go to let you read and write your emails in matrix. You can have multiple emailaccounts in different private rooms and write emails to one or multiple recipients.

<b>Matrix room:</b> <a href="https://matrix.to/#/#jojiiMail:matrix.jojii.de" target="_blank">#jojiiMail:matrix.jojii.de</a>

## Information
This bridge is currently in development. Its not 100% tested
<br>
You can run the install.sh to install it. If that doesn't work, use the steps below and contact me so I can fix it.
<br>

## Install
Clone this repository and run <code>go get</code> inside the folder to fetch the required dependencies and <code>go build -o emailbridge</code> to compile it. Afterwards execute the created binary(`./emailbridge`). Then you have to adjust the config file to make it work with your matrix server.
Invite your bridge into a private room, it will join automatically.
<br>If everyting is set up correctly, you can bridge the room by typing <code>!login</code>. Then you just have to follow the instructions. The command <code>!help</code> shows a list with available commands.<br>Creating a new private room with the bot/bridge lets you add a different email account.<br>
Using following command allows you to get an accesstoken for a given user:<br>
```bash
curl -X POST -H "Content-Type:application/json" http://<domain>:8008/_matrix/client/r0/login -d '{"type":"m.login.password","identifier":{"type":"m.id.user","user":"<USERNAME>"},"password":"<PASSWORD>"}'
```

## Note
Note: you should change the permissions of the <code>cfg.json</code> and <code>data.db</code> to <b>640</b> or <b>660</b> because they contain sensitive data, not every user should be able to read them!

## Features
- [X]  Receiving Email with IMAPs
- [X]  Use custom IMAPs Server and port
- [X]  Use the bridge with multiple email addresses
- [X]  Use the bridge with multiple user
- [X]  Ignore SSL certs if required
- [X]  Detailed error codes/logging 
- [X]  Use custom mailbox instead of INBOX
- [X]  Sending emails (to one or multiple participants)
- [X]  Use markdown (automatically translated to HTML) for writing emails (optional)
- [X]  Viewing HTML messages (as good as your matrix-client supports html)
- [X]  Attaching files sent into the bridged room

## TODO

- [ ]  Emailaddress blocklist (Ignore emails from given emailaddress)
- [ ]  System to send passwords not in plaintext
- [ ]  Add more header (CC/Bcc)
- [ ]  Update the installerscript
