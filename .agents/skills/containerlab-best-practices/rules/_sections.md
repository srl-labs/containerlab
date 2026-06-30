# Sections

This file defines all sections, their ordering, impact levels, and descriptions.
The section ID (in parentheses) is the filename prefix used to group rules.

---

## 1. User Compatibility (cli)

**Impact:** CRITICAL
**Description:** CLI flags, topology syntax, labels, state files, and generated artifacts are user-facing contracts. Breaking them silently breaks real labs and automation.

## 2. Operational Lifecycle Safety (lifecycle)

**Impact:** CRITICAL
**Description:** Deploy, destroy, apply, and reconcile act on real host and container state. They must be idempotent, context-aware, ordering-aware, and recoverable under partial failure.

## 3. Architecture and Extension Boundaries (architecture)

**Impact:** CRITICAL
**Description:** Generic code orchestrates; concrete types own their behavior. Never dispatch on a link type, node kind, or runtime name — delegate to an interface and register new types. This is the largest source of review churn.

## 4. Link, Endpoint, Node, and Runtime Contracts (contracts)

**Impact:** HIGH
**Description:** Behavior belongs to the abstraction that owns it. Links own link semantics, endpoints own endpoint-local state, nodes own kind behavior, runtimes own provider behavior.

## 5. Topology, Schema, and Docs (topology)

**Impact:** HIGH
**Description:** Topology syntax is a public API. Keep parsing, resolution, validation, inheritance, schema, docs, examples, and generated artifacts aligned.

## 6. Go Context, Errors, and Logging (go)

**Impact:** MEDIUM-HIGH
**Description:** Thread context through blocking work, return and wrap errors instead of only logging, and match existing package patterns rather than inventing abstractions.

## 7. Tests and Validation (tests)

**Impact:** MEDIUM-HIGH
**Description:** Scale test coverage with blast radius: focused unit tests for logic, interface tests for contracts, and Robot Framework integration tests for real lifecycle behavior.

---
