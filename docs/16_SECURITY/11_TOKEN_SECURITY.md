
---

# `18_SECURITY/11_TOKEN_SECURITY.md`

```markdown
# STAIR PLATFORM

Document: 11_TOKEN_SECURITY.md

ID: SEC-0011

Status: APPROVED

---

# Purpose

Определяет безопасность authentication и authorization tokens.

---

# Token Claims

Минимально необходимые claims:

Subject

Issuer

Audience

Expiration

Issued At

Token ID where required

---

# Token Validation

Перед использованием проверяются:

Signature

Issuer

Audience

Expiration

Token Type

Required Claims

---

# Token Lifetime

Access Tokens должны иметь ограниченный lifetime.

Refresh Tokens должны иметь отдельную lifecycle policy.

---

# Refresh Token Security

Refresh Token должен:

- иметь ограниченный lifetime;
- поддерживать revocation;
- быть защищен от повторного использования;
- быть связан с соответствующей session.

---

# Key Management

Signing Keys:

- хранятся в защищенном Secret Store;
- имеют rotation strategy;
- не находятся в source code.

---

# Token Scope

Token должен предоставлять только необходимые права.

---

# Rules

Нельзя доверять claims без cryptographic verification.

Нельзя использовать expired token.

---

# Acceptance Criteria

Подделанный, просроченный или отозванный token не может создать валидный Security Context.

---

APPROVED