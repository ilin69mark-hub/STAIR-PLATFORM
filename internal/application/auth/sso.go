package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"

	"stairplatform/internal/application/audit"
)

// IDTokenClaims — верифицированное содержимое id_token (EDR-0017 §3.3).
// Issuer/Audience проверяются провайдером; Email/Name используются для
// provisioning.
type IDTokenClaims struct {
	Issuer   string
	Audience string
	Subject  string
	Email    string
	Name     string
}

// OIDCProvider — порт взаимодействия с внешним IdP (Authorization Code
// flow + PKCE, OIDC Discovery, JWKS-верификация). Реализуется
// инфраструктурным слоем (изолированный HTTP-клиент).
type OIDCProvider interface {
	// Enabled возвращает true, если SSO сконфигурирован (issuer задан).
	Enabled() bool
	// Name возвращает имя провайдера (показывается на кнопке входа).
	Name() string
	// AuthCodeURL строит authorization URL с state, nonce и PKCE challenge.
	AuthCodeURL(state, nonce, codeChallenge string, codeChallengeMethod string) (string, error)
	// Exchange обменивает authorization code на токены и возвращает raw
	// id_token (идемпотентно валидируется в VerifyIDToken).
	Exchange(ctx context.Context, code, codeVerifier string) (string, error)
	// VerifyIDToken проверяет подпись (JWKS), iss, aud, exp, nonce и
	// возвращает claims.
	VerifyIDToken(ctx context.Context, rawToken, nonce string) (*IDTokenClaims, error)
}

// SsoConfig — public-конфигурация SSO для фронтенда (EDR-0017 §6).
type SsoConfig struct {
	Enabled  bool   `json:"enabled"`
	Provider string `json:"provider"`
}

// ssoTTL — срок жизни одноразового state (10 минут).
const ssoTTL = 10 * time.Minute

// randomHex возвращает n случайных байт в hex.
func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("auth: random: %w", err)
	}
	return fmt.Sprintf("%x", b), nil
}

// pkceChallengeS256 вычисляет S256 PKCE challenge от verifier.
func pkceChallengeS256(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// bcryptHash хеширует пароль (заглушка для SSO-пользователей: случайные
// данные, вход по паролю невозможен).
func bcryptHash(pw string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("auth: hash password: %w", err)
	}
	return string(h), nil
}

// SsoAuthorizeURL начинает SSO-вход (EDR-0017 §3.3, шаг 1): генерирует
// state/nonce/PKCE verifier, сохраняет одноразовое состояние и строит
// authorization URL IdP. redirect — целевой путь фронтенда после входа.
// ErrSsoNotConfigured — SSO выключен.
func (s *Service) SsoAuthorizeURL(ctx context.Context, redirect string) (string, error) {
	if s.sso == nil || !s.sso.Enabled() {
		return "", ErrSsoNotConfigured
	}
	if redirect == "" {
		redirect = "/"
	}
	state, err := randomHex(32)
	if err != nil {
		return "", err
	}
	nonce, err := randomHex(16)
	if err != nil {
		return "", err
	}
	verifier, err := randomHex(32)
	if err != nil {
		return "", err
	}
	st := &SsoState{
		StateHash:    HashToken(state),
		Nonce:        nonce,
		PKCEVerifier: verifier,
		Redirect:     redirect,
		ExpiresAt:    time.Now().UTC().Add(ssoTTL),
	}
	if err := s.repo.CreateSsoState(ctx, st); err != nil {
		return "", fmt.Errorf("auth: sso state: %w", err)
	}
	return s.sso.AuthCodeURL(state, nonce, pkceChallengeS256(verifier), "S256")
}

// SsoCallback завершает SSO-вход (EDR-0017 §3.3, шаг 3): проверяет state,
// обменивает code, верифицирует id_token и выдаёт сессию. Возвращает
// пользователя и session-токен.
func (s *Service) SsoCallback(ctx context.Context, code, state string) (*User, string, error) {
	if s.sso == nil || !s.sso.Enabled() {
		return nil, "", ErrSsoNotConfigured
	}
	if code == "" || state == "" {
		return nil, "", ErrSsoDenied
	}
	st, err := s.repo.ConsumeSsoState(ctx, HashToken(state))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			s.record(ctx, "", "", audit.ActionSsoLoginDenied, audit.ResultDenied, "invalid or expired state")
			return nil, "", ErrSsoDenied
		}
		return nil, "", err
	}
	raw, err := s.sso.Exchange(ctx, code, st.PKCEVerifier)
	if err != nil {
		s.record(ctx, "", "", audit.ActionSsoLoginDenied, audit.ResultDenied, "token exchange failed: "+err.Error())
		return nil, "", ErrSsoDenied
	}
	claims, err := s.sso.VerifyIDToken(ctx, raw, st.Nonce)
	if err != nil {
		s.record(ctx, "", "", audit.ActionSsoLoginDenied, audit.ResultDenied, "id_token rejected: "+err.Error())
		return nil, "", ErrSsoDenied
	}
	if claims.Email == "" {
		s.record(ctx, "", "", audit.ActionSsoLoginDenied, audit.ResultDenied, "no email claim")
		return nil, "", ErrSsoDenied
	}

	tenant, err := s.repo.DefaultTenant(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("auth: sso default tenant: %w", err)
	}

	name := claims.Name
	if name == "" {
		name = claims.Email
	}

	// Привязка ранее созданного identity (subject) к учётной записи.
	existing, err := s.repo.GetOAuthAccountByProviderSubject(ctx, s.sso.Name(), claims.Subject)
	if err == nil {
		u, err := s.repo.GetUserByID(ctx, existing.UserID)
		if err != nil {
			return nil, "", err
		}
		if u.Status != StatusActive {
			s.record(ctx, u.ID, u.TenantID, audit.ActionSsoLoginDenied, audit.ResultDenied, "user disabled")
			return nil, "", ErrSsoDenied
		}
		return s.loginAfterSso(ctx, u)
	}
	if !errors.Is(err, ErrNotFound) {
		return nil, "", err
	}

	// Первый вход: ищем по email (происхождение default tenant).
	u, err := s.repo.GetUserByEmail(ctx, claims.Email)
	if err == nil {
		if u.TenantID != tenant.ID {
			s.record(ctx, u.ID, u.TenantID, audit.ActionSsoLoginDenied, audit.ResultDenied, "email not in default tenant")
			return nil, "", ErrSsoDenied
		}
		if u.Status != StatusActive {
			s.record(ctx, u.ID, u.TenantID, audit.ActionSsoLoginDenied, audit.ResultDenied, "user disabled")
			return nil, "", ErrSsoDenied
		}
		acct := &OAuthAccount{Provider: s.sso.Name(), Subject: claims.Subject, UserID: u.ID}
		if err := s.repo.CreateOAuthAccount(ctx, acct); err != nil {
			if errors.Is(err, ErrOAuthExists) {
				// Гонка: повторный колбэк (повтор state-невозможен) либо
				// параллельный первый вход — считаем успешным.
				return s.loginAfterSso(ctx, u)
			}
			return nil, "", err
		}
		s.record(ctx, u.ID, u.TenantID, audit.ActionSsoLinked, audit.ResultOK, "linked "+s.sso.Name()+" to "+claims.Email)
		return s.loginAfterSso(ctx, u)
	}
	if !errors.Is(err, ErrNotFound) {
		return nil, "", err
	}

	// Автосоздание учётной записи (default tenant, роль user). Пароль намеренно
	// случайный bcrypt-заглушка (схема users.password_hash NOT NULL); вход по
	// паролю для SSO-пользователя невозможен (Login сверит с «неизвестным»).
	stub, err := randomHex(32)
	if err != nil {
		return nil, "", err
	}
	hash, err := bcryptHash(stub)
	if err != nil {
		return nil, "", err
	}
	nu := &User{
		TenantID:     tenant.ID,
		Email:        claims.Email,
		Name:         name,
		PasswordHash: hash,
		Role:         RoleUser,
		Status:       StatusActive,
	}
	if err := s.repo.CreateUser(ctx, nu); err != nil {
		if errors.Is(err, ErrEmailExists) {
			// Email занят между колбэком и созданием — привязываем.
			nu, err = s.repo.GetUserByEmail(ctx, claims.Email)
			if err != nil {
				return nil, "", err
			}
		} else {
			return nil, "", fmt.Errorf("auth: sso create user: %w", err)
		}
	}
	acct := &OAuthAccount{Provider: s.sso.Name(), Subject: claims.Subject, UserID: nu.ID}
	if err := s.repo.CreateOAuthAccount(ctx, acct); err != nil {
		if errors.Is(err, ErrOAuthExists) {
			// Гонка первого входа: расцениваем как успех (субъект уже наш).
			return s.loginAfterSso(ctx, nu)
		}
		return nil, "", err
	}
	s.record(ctx, nu.ID, nu.TenantID, audit.ActionSsoLinked, audit.ResultOK, "provisioned "+claims.Email+" via "+s.sso.Name())
	return s.loginAfterSso(ctx, nu)
}

// loginAfterSso создаёт сессию и аудит-событие sso.login.
func (s *Service) loginAfterSso(ctx context.Context, u *User) (*User, string, error) {
	token, err := newToken()
	if err != nil {
		return nil, "", err
	}
	sess := &Session{
		UserID:    u.ID,
		TokenHash: HashToken(token),
		ExpiresAt: time.Now().UTC().Add(s.sessionTTLFor(ctx, u.TenantID)),
	}
	if err := s.repo.CreateSession(ctx, sess); err != nil {
		return nil, "", fmt.Errorf("auth: sso create session: %w", err)
	}
	s.record(ctx, u.ID, u.TenantID, audit.ActionSsoLogin, audit.ResultOK, "sso login ok")
	return u, token, nil
}

// SsoEnabled возвращает публичную конфигурацию SSO.
func (s *Service) SsoEnabled() SsoConfig {
	if s.sso == nil {
		return SsoConfig{}
	}
	return SsoConfig{Enabled: s.sso.Enabled(), Provider: s.sso.Name()}
}

// WithOIDCProvider подключает OIDC-провайдера к сервису (EDR-0017).
// Идемпотентно: повторный вызов заменяет конфигурацию.
func (s *Service) WithOIDCProvider(p OIDCProvider) *Service {
	s.sso = p
	return s
}
