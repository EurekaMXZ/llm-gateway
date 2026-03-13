# M2 Permission and Isolation Test Matrix

## Scope

This matrix defines baseline executable test scenarios for identity and apikey services in M2.

## Identity Hierarchy Cases

| Case ID | Actor Role | Resource Owner | Expected |
| --- | --- | --- | --- |
| ID-PERM-001 | superuser | any user | allow |
| ID-PERM-002 | administrator | self | allow |
| ID-PERM-003 | administrator | descendant | allow |
| ID-PERM-004 | administrator | peer subtree | deny |
| ID-PERM-005 | regular_user | self | allow |
| ID-PERM-006 | regular_user | other user | deny |

## Authentication Cases

| Case ID | Scenario | Expected |
| --- | --- | --- |
| ID-AUTH-001 | valid username/password | token issued |
| ID-AUTH-002 | invalid password | 401 |
| ID-AUTH-003 | malformed bearer token | 401 |
| ID-AUTH-004 | valid bearer token | user context returned |

## API Key Lifecycle Cases

| Case ID | Scenario | Expected |
| --- | --- | --- |
| KEY-LIFE-001 | create key | plaintext key returned once |
| KEY-LIFE-002 | disable key | subsequent validation denied (`key_disabled`) |
| KEY-LIFE-003 | re-enable key | validation allowed again |
| KEY-LIFE-004 | expired key validation | denied (`key_expired`) |

## Model Whitelist Cases

| Case ID | Scenario | Expected |
| --- | --- | --- |
| KEY-MODEL-001 | allowed model | valid |
| KEY-MODEL-002 | non-whitelisted model | denied (`model_forbidden`) |
| KEY-MODEL-003 | empty whitelist + model | valid (no model restriction) |

## Execution Baseline

- Unit tests live under:
  - `backend/services/identity-service/internal/app/service_test.go`
  - `backend/services/apikey-service/internal/app/service_test.go`
- Integration tests against HTTP routes are planned in next M2 iteration.
