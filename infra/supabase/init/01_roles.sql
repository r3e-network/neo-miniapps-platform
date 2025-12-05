-- Basic roles for PostgREST
create role anon noinherit;
create role service_role noinherit;
create role authenticated noinherit;

-- Auth schema (used by GoTrue)
create schema if not exists auth;

-- Ensure superuser role "postgres" exists for migrations expecting it.
do $$
begin
    if not exists (select 1 from pg_roles where rolname = 'postgres') then
        create role postgres superuser login;
    end if;
end
$$;

-- Core auth enums in auth schema (used by later migrations)
do $$
begin
    create type auth.factor_type as enum('totp', 'webauthn');
exception
    when duplicate_object then null;
end
$$;

do $$
begin
    create type auth.factor_status as enum('unverified', 'verified');
exception
    when duplicate_object then null;
end
$$;

do $$
begin
    create type auth.aal_level as enum('aal1', 'aal2', 'aal3');
exception
    when duplicate_object then null;
end
$$;

do $$
begin
    create type auth.code_challenge_method as enum('s256', 'plain');
exception
    when duplicate_object then null;
end
$$;

grant usage on schema public to anon, service_role, authenticated;
grant usage on schema auth to anon, service_role, authenticated;

-- Default RLS off; services enforce via TEE
alter default privileges in schema public grant select, insert, update, delete on tables to anon, service_role, authenticated;
alter default privileges in schema auth grant select, insert, update, delete on tables to anon, service_role, authenticated;
