-- Rollback Migration 000003: Add AI Agent & MCP Platform Tables

DROP TABLE IF EXISTS mcp_audit_logs CASCADE;
DROP TABLE IF EXISTS mcp_permissions CASCADE;
DROP TABLE IF EXISTS mcp_tools CASCADE;
DROP TABLE IF EXISTS mcp_connections CASCADE;
DROP TABLE IF EXISTS mcp_servers CASCADE;
DROP TABLE IF EXISTS ai_agent_runs CASCADE;
DROP TABLE IF EXISTS ai_tool_invocations CASCADE;
DROP TABLE IF EXISTS ai_approvals CASCADE;
DROP TABLE IF EXISTS ai_plan_resources CASCADE;
DROP TABLE IF EXISTS ai_resource_plans CASCADE;
DROP TABLE IF EXISTS ai_conversation_state CASCADE;
DROP TABLE IF EXISTS ai_messages CASCADE;
DROP TABLE IF EXISTS ai_conversations CASCADE;
