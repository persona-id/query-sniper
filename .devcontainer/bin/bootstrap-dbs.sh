#!/usr/bin/env bash
set -eou pipefail

# Bootstrap the databases. Add a replication user, enable GTID, and create a test table.
mysql -hdev-db-primary -P3306 -uroot -proot << EOF
CREATE DATABASE IF NOT EXISTS \`web-us1\`;

-- replication user for the replication process.
ALTER USER 'replication'@'%' IDENTIFIED WITH 'caching_sha2_password' BY 'replication';
GRANT REPLICATION SLAVE ON *.* TO 'replication'@'%';

-- sniper user is used by the process to find and kill long queries.
CREATE USER IF NOT EXISTS 'sniper'@'%' IDENTIFIED WITH 'caching_sha2_password' BY 'sniper';
GRANT PROCESS, CONNECTION_ADMIN ON *.* TO 'sniper'@'%';

-- web-us1 user is a test user.
CREATE USER IF NOT EXISTS \`web-us1\`@'%' IDENTIFIED WITH 'caching_sha2_password' BY 'web-us1';
GRANT ALL PRIVILEGES ON \`web-us1\`.* TO 'web-us1'@'%';

FLUSH PRIVILEGES;

USE \`web-us1\`;

CREATE TABLE IF NOT EXISTS config_flags (
  id INT PRIMARY KEY AUTO_INCREMENT,
  config_key VARCHAR(255),
  config_value VARCHAR(255),
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

-- just some sample data to verify replication, this isn't actually used in this project
INSERT INTO config_flags (config_key, config_value) VALUES
('sniper:us1-primary', 'enabled:true'),
('sniper:us1-replica0', 'enabled:true'),
('sniper:us1-replica1', 'enabled:true'),
('sniper:us2-primary', 'enabled:false'),
('sniper:us2-replica0', 'enabled:false'),
('sniper:us2-replica1', 'enabled:false');
EOF

# Start replication.
mysql -hdev-db-replica0 -P3306 -uroot -proot << EOF
STOP REPLICA;

CHANGE REPLICATION SOURCE TO
  SOURCE_HOST='dev-db-primary',
  SOURCE_USER='replication',
  SOURCE_PASSWORD='replication',
  SOURCE_PORT=3306,
  SOURCE_SSL=1,
  SOURCE_AUTO_POSITION=1;

START REPLICA;
EOF

mysql -hdev-db-replica1 -P3306 -uroot -proot << EOF
STOP REPLICA;

CHANGE REPLICATION SOURCE TO
  SOURCE_HOST='dev-db-primary',
  SOURCE_USER='replication',
  SOURCE_PASSWORD='replication',
  SOURCE_PORT=3306,
  SOURCE_SSL=1,
  SOURCE_AUTO_POSITION=1;

START REPLICA;
EOF