-- Seed 5 local-dev contestants with deterministic UUIDs and API key hashes.
-- The plaintext keys are key-alice-0001 .. key-eve-0005; only their SHA-256
-- hashes are stored. Safe to re-run (ON CONFLICT DO NOTHING).
INSERT INTO contestants (id, name, email, api_key_hash) VALUES
    ('11111111-1111-4111-8111-111111111111', 'Alice', 'alice@example.com', '01f9350b55022160f9b24feea1557eeec5995bbd4b459d2d107ee247b2b17375'),
    ('22222222-2222-4222-8222-222222222222', 'Bob',   'bob@example.com',   '4ead32619d45c41952a53c1c6ef77ec7ca83f2d03e18abc8cb339f3d28e3ecec'),
    ('33333333-3333-4333-8333-333333333333', 'Carol', 'carol@example.com', '97385903a9bf179686c78b4d92e5ce2efb98123e8a9db8c3a69696e0a3980312'),
    ('44444444-4444-4444-8444-444444444444', 'Dave',  'dave@example.com',  '63e4a50b751506036063c83a06927a217e67c466bc495e187deef5cd8ab4c3e2'),
    ('55555555-5555-4555-8555-555555555555', 'Eve',   'eve@example.com',   '27755369a1432103597857c798bb54d24498c9cd07c10895d450f91cc34c2703')
ON CONFLICT (id) DO NOTHING;
