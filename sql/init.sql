-- init.sql is a postgresl pl/pgsql script to
-- setup a fresh database for the irc logger

CREATE TABLE IF NOT EXISTS network (
  id   SERIAL PRIMARY KEY,
  name TEXT UNIQUE NOT NULL
);

CREATE TABLE IF NOT EXISTS message (
  id     SERIAL PRIMARY KEY,
  net    INT REFERENCES public.network (id) NOT NULL,
  time   TIMESTAMP WITHOUT TIME ZONE,
  -- max nick and chan length varies by ircds , set to something fairly safe. otherwise truncation can occur
  "from" TEXT,
  "to"   TEXT,
  line   TEXT
);

CREATE OR REPLACE VIEW newest_msgs_by_network AS
  SELECT
    n.name,
    m.time,
    m.from,
    m.to,
    m.line
  FROM network n
    INNER JOIN message m ON n.id = m.net
  GROUP BY n.name, m.time, m.from, m.to, m.line
  ORDER BY m.time DESC;

-- get_network takes network name as a string and will create it if it doesnt exist,
-- this is very loosey-goosey and should only be used during bulk imports
CREATE OR REPLACE FUNCTION get_network_id(netname TEXT)
  RETURNS INT AS $$
DECLARE
  netid INT := NULL;
BEGIN
  SELECT id
  INTO netid
  FROM network n
  WHERE n.name = netname;
  IF netid IS NULL
  THEN
    WITH added_id AS (INSERT INTO network (name) VALUES (netname)
    RETURNING id)
    SELECT id
    INTO netid
    FROM added_id;
  END IF;
  RETURN netid;
END;
$$ LANGUAGE PLPGSQL;

CREATE OR REPLACE FUNCTION add_msg(network_id INT, mtime TIMESTAMP, f TEXT, target TEXT, msg TEXT)
  RETURNS INT
AS $$
DECLARE
  msgid INT := NULL;
BEGIN
  WITH new_msg_q AS (INSERT INTO message (net, time, "from", "to", line)
    SELECT
      network_id,
      mtime,
      f,
      target,
      msg
  RETURNING id)
  SELECT INTO msgid id
  FROM new_msg_q;
  RETURN msgid;
END;
$$
LANGUAGE PLPGSQL;
