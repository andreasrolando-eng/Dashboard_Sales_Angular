-- +goose Up
-- Login lokal (email + password). password_hash memakai bcrypt; NULL = user
-- belum punya password (belum bisa login lewat form, mis. menunggu SSO).
alter table users add column password_hash text;
alter table users add column last_login_at timestamptz;

-- Sesi login di sisi server. Cookie hanya membawa token acak; yang disimpan
-- di sini hanya SHA-256-nya, jadi kebocoran tabel tidak membocorkan sesi aktif.
create table sessions (
  id bigserial primary key,
  token_hash text not null unique,
  user_id bigint not null references users(id) on delete cascade,
  created_at timestamptz not null default now(),
  expires_at timestamptz not null,
  user_agent text,
  ip text
);
create index sessions_user_id_idx on sessions (user_id);
create index sessions_expires_at_idx on sessions (expires_at);

-- +goose Down
drop table sessions;
alter table users drop column last_login_at;
alter table users drop column password_hash;
