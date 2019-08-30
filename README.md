# Matrix-EmailBridge
A matrix-bridge written in Go to let you read and write your emails in matrix. You can have multiple emailaccounts in different private rooms, write emails to multiple 

<b>Matrix room:</b> <a href="https://matrix.to/#/#jojiiMail:matrix.jojii.de" target="_blank">#jojiiMail:matrix.jojii.de</a>

## Information
This bridge is currently in development. Its not 100% tested
<br>
You can run the install.sh to install it. If that does'nt work, use the steps below and contact me to fix it
<br>

## Install
Just run <code>go get</code> to fetch the required dependencies and <code>go build</code> inside the folder and execute the created binary. Then you have to adjust the config file to make it work with your matrix server.
Invite your bridge into a private room, it will join automatically.
<br>If everyting is set up correctly, you can bridge the room by typing <code>!login</code>. Then you just have to follow the instructions. Typing <code>!help</code> shows a list with available commands.<br>Creating a new private room with the bot/bridge lets you add a different email account.<br>

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

## TODO
- [ ]  Attaching files sent into bridge
- [ ]  System to send passwords not in plaintext
- [ ]  Add more header (CC/Bcc)