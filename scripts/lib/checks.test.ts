import { expect, test } from "bun:test";
import { postgresFailure, redisVersionProblem } from "./checks.ts";

test("Redis 7 or newer is fine; the login limiter needs EXPIRE ... NX (Redis 7.0)", () => {
  expect(redisVersionProblem("# Server\r\nredis_version:8.10.2\r\nos:Linux")).toBeNull();
  expect(redisVersionProblem("redis_version:7.0.0")).toBeNull();
  expect(redisVersionProblem("redis_version:7.4.1")).toBeNull();
});

test("Redis 6 is refused with the reason and the way out", () => {
  const problem = redisVersionProblem("redis_version:6.2.13");
  expect(problem).toContain("6.2.13");
  expect(problem).toContain("7.0");
  expect(problem).toContain("Memurai");
});

test("a reply that is not INFO output is not trusted", () => {
  expect(redisVersionProblem("-NOAUTH Authentication required.")).toContain("could not read the version");
});

test("Postgres errors say what to do, by SQLSTATE", () => {
  expect(postgresFailure({ errno: "28P01", message: "password authentication failed for user \"tepati\"" }, "postgres://tepati@localhost:5440/tepati")).toContain("password");
  expect(postgresFailure({ errno: "3D000", message: 'database "tepati" does not exist' }, "postgres://tepati@localhost:5440/tepati")).toContain("createdb");
  expect(postgresFailure({ code: "ERR_POSTGRES_CONNECTION_REFUSED", message: "Failed to connect" }, "postgres://tepati@localhost:5440/tepati")).toContain("localhost:5440");
});

test("an unknown Postgres error is passed through rather than guessed at", () => {
  expect(postgresFailure(new Error("boom"), "postgres://u@h:1/d")).toContain("boom");
});

test("the connection string's password never appears in a message", () => {
  const msg = postgresFailure({ errno: "28P01", message: "bad" }, "postgres://tepati:s3cret@localhost:5440/tepati");
  expect(msg).not.toContain("s3cret");
});
