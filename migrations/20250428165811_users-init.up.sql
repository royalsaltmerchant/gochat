CREATE TABLE users (
  id integer primary key,
  username text not null,
  email text not null unique,
  password text not null
);