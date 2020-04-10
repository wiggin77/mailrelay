# mailrelay

[![Build Status](https://travis-ci.org/wiggin77/mailrelay.svg?branch=master)](https://travis-ci.org/wiggin77/mailrelay)

`mailrelay` is a simple mail relay that can take unauthenticated SMTP emails (e.g. over port 25) and relay them to authenticated, TLS-enabled SMTP servers. Plus it's easy to configure.

## Use case

Some older appliances such as scanners, multi-function printers, RAID cards or NAS boxes with monitoring, can only send email without any authentication or encryption over port 25. `mailrelay` can send those emails to your Gmail, Fastmail or other provider.

Run `mailrelay` on a local PC and set your device (e.g. scanner) to send mail to that PC.

`mailrelay` is written in Go, and can be compiled for any Go supported platform including Linux, MacOS, Windows.

## Example (Linux)

On local PC (192.168.1.54) create file `/etc/mailrelay.json` with contents:

/etc/mailrelay.json

```json
{
    "smtp_server":   "smtp.fastmail.com",
    "smtp_port":     465,
    "smtp_username": "username@fastmail.com",
    "smtp_password": "secretAppPassword",
    "local_listen_ip": "0.0.0.0",
    "local_listen_port": 2525,
    "allowed_hosts": ["*"]
}
```

Run `mailrelay`,

```Bash
./mailrelay
```

Default location for configuration file is `/etc/mailrelay.json` but can be changed via `--config` flag. For example,

```bash
mailrelay --config=/home/myname/mailrelay.json
```

Configure your scanner or other device to send SMTP mail to server `192.168.1.54:2525`. Each email will be relayed to `smtp.fastmail.com` using the credentials above, including any file attachments.

## Example 2 (Linux - Systemd service)

Create configuration file as above, and also create,

/etc/systemd/system/mailrelay.service

```ini
[Unit]
Description=Mail Relay Service
After=network.target
StartLimitIntervalSec=0

[Service]
Type=simple
Restart=always
RestartSec=5
ExecStart=/usr/local/bin/mailrelay

[Install]
WantedBy=multi-user.target
```

Copy `mailrelay` to `/usr/local/bin/`.

Run,

```Bash
sudo systemctl start mailrelay
sudo systemctl enable mailrelay
```

Now `mailrelay` runs as a service daemon and will automatically start after reboot.

## Example 3 (Docker)

edit mailrelay.json as needed then build container and run it

```
docker build  -t mailrelay .
docker run -v $PWD/mailrelay.json:/etc/mailrelay.json mailrelay
```

## Feedback

Send any questions or comments to wiggin77@warpmail.net