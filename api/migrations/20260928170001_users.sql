-- +goose Up
create table users (
  id bigserial primary key,
  email text not null unique,
  name text,
  -- OIDC subject (operations-sso) -- nullable, filled in on first SSO login.
  -- SSO itself isn't wired yet, so an admin can pre-provision an email here
  -- before the person has ever logged in.
  sub text unique,
  is_admin boolean not null default false,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

insert into users (email, is_admin)
values ('andreas.rolando@esb.co.id', true)
on conflict (email) do update set is_admin = true;

-- +goose Down
drop table users;
