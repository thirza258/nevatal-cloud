-- Migration 000004 Down: Drop Game Server Hosting Tables

DROP TABLE IF EXISTS game_server_audit_logs;
DROP TABLE IF EXISTS game_server_backups;
DROP TABLE IF EXISTS game_server_worlds;
DROP TABLE IF EXISTS game_server_mods;
DROP TABLE IF EXISTS game_server_plugins;
DROP TABLE IF EXISTS game_server_players;
DROP TABLE IF EXISTS game_server_configurations;
DROP TABLE IF EXISTS game_server_ports;
DROP TABLE IF EXISTS game_server_versions;
DROP TABLE IF EXISTS game_server_templates;
DROP TABLE IF EXISTS game_servers;
