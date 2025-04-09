#!/bin/bash
set -eou pipefail

mysql -hdb-primary -P3306 -uroot -proot << EOF
ALTER USER 'test'@'%' IDENTIFIED WITH 'caching_sha2_password' BY 'test';
GRANT REPLICATION SLAVE ON *.* TO 'test'@'%';
FLUSH PRIVILEGES;

USE test;

CREATE TABLE IF NOT EXISTS configs (
  id INT PRIMARY KEY AUTO_INCREMENT,
  config_key VARCHAR(255),
  config_value VARCHAR(255),
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

INSERT INTO configs (config_key, config_value) VALUES ('org:persona', 'enabled:true');
EOF

mysql -hdb-replica -P3306 -uroot -proot << EOF
STOP REPLICA;

CHANGE REPLICATION SOURCE TO
  SOURCE_HOST='db-primary',
  SOURCE_USER='test',
  SOURCE_PASSWORD='test',
  SOURCE_PORT=3306,
  SOURCE_SSL=1,
  SOURCE_AUTO_POSITION=1;

START REPLICA;
EOF
