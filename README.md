# UTC Alarm Receiver

_utcar_ acts as a central station to a ATS alarm system and has been tested using an ATS2000IP system.  It might work with other variants as well.  The communication between the alarm system and utcar is using OH+XSIA protocol.  I'm not an expert, but I assume that any system that uses this protocol should be compatible (naive? me?).

My use case:
* leverage the alarms systems events (e.g. motion, window sensors) as input to home automation

Features:
* receives OH+XSIA messages (3DES) and acknowledges them immediately
* parses every SIA-DCS message: all SIA event codes (with descriptions), zones, areas, users, texts, several events per message
* optionally pushes zone alarms/restorals to openHAB (`--addr`)
* optionally stores every message and event in an SQLite database (`--db`)
* optionally forwards the messages to one or more monitoring centers as SIA DC-09 frames over TCP (`--forward`)

## Configure your alarm system

Your alarm is typically configured to send messages to a central station in case of alarm, fire, etc...  The link is checked for health by sending periodic heartbeat messages.  In my case, these heartbeat messages are sent every minute.

For _utcar_ to work, we'll configure it in the alarm system as a new central station.  Since we don't want to interrupt messages going to the first central station (the one calling the police in case of an alarm), we need to go through a few steps to make this work.  Let's assume we want to get notified when a motion sensor is triggered.

1. Create a new area that we'll use with _utcar_.  This allows us to keep automation for _utcar_ separate from the actual alarms going to the formal central station.  Let's pick area 4.
  ![alt text](https://github.com/tdeckers/utcar/raw/master/img/area.png)
2. Configure a filter that follows the motion detector
  ![alt text](https://github.com/tdeckers/utcar/raw/master/img/filter.png)
3. Configure an output to follow the filter
  ![alt text](https://github.com/tdeckers/utcar/raw/master/img/output.png)
4. Configure a new zone, and configure the output as its _virtual zone_.  Configure it in area 4.
  ![alt text](https://github.com/tdeckers/utcar/raw/master/img/zone.png)
5. Configure a new central station (IP based), configure it with the host IP address of the machine where you'll be running _utcar_.  Also pick a port number, by default _utcar_ runs on port 12300.  Associate this central station with area 4.
  ![alt text](https://github.com/tdeckers/utcar/raw/master/img/central_station.png)


## Running utcar

	Usage:
	  utcar [flags]

	Flags:
	      --addr string                Target addr (e.g. http://openhab.local:8080)
	      --db string                  SQLite database file (default: no database)
	      --db-heartbeats              Store heartbeat messages in the database
	      --debug int                  Debug server port number (default: no debug server)
	      --forward strings            Monitoring center to forward messages to: host:port, tcp://host:port or tcp://host:port?format=raw (repeatable or comma separated)
	      --forward-heartbeats         Forward heartbeats as DC-09 NULL (link test) messages (default true)
	      --forward-queue int          Messages waiting to be forwarded, per monitoring center (default 1000)
	      --forward-timeout duration   Monitoring center connect and response timeout (default 10s)
	  -h, --help                       help for utcar
	      --port int                   Listen port number (default 12300)
	      --pwd string                 Target password
	      --user string                Target username

Every flag can also be set with an environment variable: `UTCAR_` + the flag
name in upper case, with `-` replaced by `_`:

| Flag | Environment variable | Default |
|---|---|---|
| `--port` | `UTCAR_PORT` | `12300` |
| `--addr`, `--user`, `--pwd` | `UTCAR_ADDR`, `UTCAR_USER`, `UTCAR_PWD` | (no openHAB) |
| `--debug` | `UTCAR_DEBUG` | `0` (no debug server) |
| `--db` | `UTCAR_DB` | (no database) |
| `--db-heartbeats` | `UTCAR_DB_HEARTBEATS` | `false` |
| `--forward` | `UTCAR_FORWARD` (comma separated list) | (no forwarding) |
| `--forward-timeout` | `UTCAR_FORWARD_TIMEOUT` | `10s` |
| `--forward-queue` | `UTCAR_FORWARD_QUEUE` | `1000` |
| `--forward-heartbeats` | `UTCAR_FORWARD_HEARTBEATS` | `true` |

Example:

	./utcar -port=10000 --addr=https://localhost:8443 --user=yourname --pwd=yourpass
	2014/09/25 06:40:32 Listing on port 10000...
	2014/09/25 06:40:32 Pushing to localhost:8443

Example with environment variables:

	UTCAR_PORT=12000 ./utcar
	2020/04/25 12:37:38 Listing on port 12000...

Example with a database and two monitoring centers:

	./utcar --db /var/lib/utcar/utcar.db --forward 10.0.0.1:5000 --forward tcp://10.0.0.2:5001
	2026/10/07 22:00:00 Listing on port 12300...
	2026/10/07 22:00:00 Storing messages in /var/lib/utcar/utcar.db (heartbeats: false)
	2026/10/07 22:00:00 Forwarding to 10.0.0.1:5000 (format dc09, heartbeats: true)
	2026/10/07 22:00:00 Forwarding to 10.0.0.2:5001 (format dc09, heartbeats: true)

The same with environment variables:

	UTCAR_DB=/var/lib/utcar/utcar.db UTCAR_FORWARD=10.0.0.1:5000,tcp://10.0.0.2:5001 ./utcar

_utcar_ listens by default on port number 12300, can be set on command line (`--port`)

## Messages

The alarm must be configured to send OH+XSIA messages. In that case it will send two types of messages: heartbeats and (X)SIA messages.
The heartbeats look like this:

	SR0001L0001    001465XX    [ID5B9490D8]

Heartbeats are logged, optionally stored in the database (`--db-heartbeats`)
and forwarded to the monitoring centers as DC-09 `NULL` (link test) messages
(unless `--forward-heartbeats=false`).

The (X)SIA messages are more interesting, the look like this:

	01010053"SIA-DCS"0007R0073L0011[#001365|NUA021*'detector hall'NM]7C9677F21948CC12|#001365

This is a message to indicate activation (UA) of a motion sensor in zone 21 (detector in hall).

Every SIA-DCS message is parsed completely: any SIA event code (described from
the SIA DC-03 code table, unknown codes are kept), zones of any length, several
events in one message (`NUA021/UA022`, several `[...]` blocks), the area
(`ri`), user (`id`) and time (`ti`) modifiers and the text (`*'...'`). The
message and each event are logged:

	SIA-DCS message from 192.168.1.10:41166: seq 0008, receiver 0075, line 0001, account 001465, 2 event(s)
	  Event: UA (Untyped Zone Alarm) zone 021 text 'hall'
	  Event: UR (Untyped Zone Restoral) zone 022

A message that can't be recognized is acknowledged to the alarm, logged as a
warning and stored in the database (kind `unknown`); it never stops _utcar_.

The alarm always gets its ACK immediately: storing and forwarding happen after
that, forwarding in the background.

## Pushing to openHAB

If no `--addr` is provided, nothing is pushed.

if a `--addr` parameter is provided, then _utcar_ will POST a message to an HTTP endpoint. Right now, this is customized for Openhab - might need to generalize this later.

URL for the POST: `<addr>/rest/items/al_{item}/state`, where item is the zone received from the alarm.

For the example above, an HTTP POST with body ON is sent to:

	<addr>/rest/items/al_021/state

You can provide `--user` and `--pwd` to provide basic authentication.

If an (X)SIA message of UR is received, an OFF message is sent. Every event of
a message is pushed; for other event codes the push is skipped with a
`Push error: Unsupported SIA command` log line.

## Storing messages in a database

With `--db <file>` every received message is stored in an SQLite database (the
file is created if it doesn't exist, the directory must exist). Heartbeats are
only stored with `--db-heartbeats`. A database error is logged, the message is
still pushed and forwarded.

Tables:

* `messages` – one row per message: `id`, `received_at` (UTC,
  `2006-01-02T15:04:05.000Z`), `remote` (address of the alarm), `kind`
  (`sia`, `heartbeat` or `unknown`), `protocol`, `sequence`, `receiver`,
  `line`, `account`, `raw` (the decrypted message), `parse_error`.
* `events` – one row per SIA event: `id`, `message_id` (→ `messages.id`),
  `code`, `description`, `zone`, `area`, `user_id`, `event_time`, `text`.

Example queries (`sqlite3 utcar.db`):

```sql
-- last 20 events
SELECT m.received_at, m.account, e.code, e.description, e.zone, e.text
FROM events e JOIN messages m ON m.id = e.message_id
ORDER BY e.id DESC LIMIT 20;

-- burglary/untyped alarms and restorals of zone 021 today (UTC)
SELECT m.received_at, e.code, e.description
FROM events e JOIN messages m ON m.id = e.message_id
WHERE e.zone = '021' AND e.code IN ('BA', 'BR', 'UA', 'UR')
  AND m.received_at >= date('now')
ORDER BY m.received_at;

-- number of events per code
SELECT code, description, COUNT(*) AS n
FROM events GROUP BY code, description ORDER BY n DESC;

-- unrecognized messages
SELECT received_at, remote, parse_error, raw
FROM messages WHERE kind = 'unknown' ORDER BY id DESC;
```

The database uses WAL mode (files `<db>-wal` and `<db>-shm` next to it). It can
be read with `sqlite3` while _utcar_ is running.

## Forwarding to monitoring centers

With `--forward` the messages are forwarded to one or more monitoring centers,
unencrypted, over TCP, as SIA DC-09 frames
(`LF crc 0LLL "SIA-DCS"seq Rrcvr Lline #acct[data] CR`, CRC-16/ARC). Use the
flag several times, or separate the centers with a comma (always with a comma
in `UTCAR_FORWARD`):

	--forward host:port
	--forward tcp://host:port
	--forward tcp://host:port?format=raw

* `format=dc09` (default): the frame is built from the parsed message, with a
  correct CRC and length. Unrecognized messages are not forwarded.
* `format=raw`: the decrypted message is sent as it was received
  (`LF + message + CR`), unrecognized messages included. Note that the ATS
  message is **not** a valid DC-09 frame (its first 8 characters, e.g.
  `01010053`, are not the DC-09 CRC and length), so use `raw` only for a
  center that accepts the same format as the ATS (OH+XSIA).

Every center has its own queue (`--forward-queue` messages, default 1000) and
its own connection; a center that is down doesn't delay the alarm or the other
centers, and the order of the messages to one center is kept. Every message
uses a new TCP connection and waits for the center's response
(`--forward-timeout`, default 10s, for connecting and for the response):

* `ACK` – delivered.
* Timeout or connection error – an event message is retried until it is
  delivered (backoff 1s, doubling up to 60s), however long the center is down.
* `NAK` – retried the same way, but after 5 consecutive `NAK`s the message is
  dropped (logged), so that a message the center keeps rejecting doesn't block
  the queue.
* `DUH` – the center doesn't support the message; it is dropped (logged).

A heartbeat (`NULL`) and an unrecognized message (`raw` format) are tried only
once.

Heartbeats are forwarded as DC-09 `NULL` messages (numbered 0001–9999; in
`raw` format as received) unless `--forward-heartbeats=false`.

**The queue is kept in memory only**: messages that are not yet delivered are
lost when _utcar_ stops or restarts (they remain in the database, if one is
used). A full queue drops new messages with a warning in the log.

Example log:

	Forwarder 10.0.0.2:5001: SIA-DCS message 0008 (account 001465) not delivered (dial tcp 10.0.0.2:5001: connect: connection refused), retrying in 1s
	Forwarder 10.0.0.1:5000: delivered SIA-DCS message 0008 (account 001465) (attempt 1)
	Forwarder 10.0.0.2:5001: delivered SIA-DCS message 0008 (account 001465) (attempt 3)

## Running in a container

You can also run utcar in a container:

	docker run -p 10000:10000 tdeckers/utcar --port=10000 --addr=https://localhost:8443

Or alternatively:

	docker run -p 12300:12300 -e UTCAR_ADDR=https://localhost:8443 tdeckers/utcar

With a database and two monitoring centers (the database is kept in the
`utcar-data` volume, mounted on `/data`):

	docker run -d --name utcar -p 12300:12300 -v utcar-data:/data \
	    -e UTCAR_DB=/data/utcar.db \
	    -e UTCAR_FORWARD=10.0.0.1:5000,10.0.0.2:5001 \
	    utcar

Or with a host directory: `-v /var/lib/utcar:/data`.

# Building

Go 1.24 or newer is needed. The SQLite driver (`modernc.org/sqlite`) is pure
Go, so no C compiler is needed and _utcar_ can be built with `CGO_ENABLED=0`
for any platform:

	go build
	# Windows 64-bit
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build
	# Raspberry Pi
	CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=6 go build -o utcar_linux_arm6
	# Raspberry Pi (64-bit OS)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build

Tests:

	go vet ./...
	go test ./...

Note: 3 versions are provided: for Windows 64, arm32 (for raspberry pi) and for linux64. See releases.

## Building a container
(thanks: https://rollout.io/blog/building-minimal-docker-containers-for-go-applications/)

First build a statically linked executable:

	CGO_ENABLED=0 GOARCH=amd64 GOOS=linux go build
	cp /etc/ssl/certs/ca-certificates.crt .
	docker build -t utcar .

Then run the container:

	docker run -p 12300:12300 utcar --addr=http://localhost:8080

Or

	docker run -p 12300:12300 -e UTCAR_ADDR=http://localhost:8080 utcar

The image declares a volume on `/data` for the database (`--db /data/utcar.db`).

Credits: Thanks to Dirk @ OP for his help on the ATS configuration.
