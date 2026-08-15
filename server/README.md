# Central Server

This folder contains the central gateway/server for the SASMAN tunnel system.

## Purpose
- receive tunnel registrations from customer agents
- route public traffic by subdomain
- manage SQLite-backed metadata for licenses and subdomains

## Run

```bash
go run ./server
```
