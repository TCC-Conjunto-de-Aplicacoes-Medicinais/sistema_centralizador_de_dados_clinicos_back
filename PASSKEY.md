# Passkey (WebAuthn) do paciente: vínculo back ↔ app

Este documento é para quem sobe o back e o app. O back é o centralizador (`api.pohinc.com.br`).

## Variáveis de ambiente do back

| Variável | Valor | Observação |
|---|---|---|
| `WEBAUTHN_RP_ID` | `app.pohinc.com.br` | Domínio da passkey. Tem que ser o domínio do app web. |
| `WEBAUTHN_ORIGINS` | `https://app.pohinc.com.br` | Origens permitidas, separadas por vírgula. |
| `WEBAUTHN_ORIGINS` (Android nativo) | `...,android:apk-key-hash:<HASH>` | Só quando o app Android nativo for gerado. O hash vem do certificado de assinatura. |

Se `WEBAUTHN_RP_ID` e `WEBAUTHN_ORIGINS` não forem definidos, o back usa `localhost` e não funciona em produção.

As tabelas `patient_passkey` e `patient_recovery` são criadas sozinhas no startup (AutoMigrate).

## Endpoints novos

Todos ficam em `/api/passkeys/...`.

- `POST /register/begin` e `POST /register/finish`: cadastro da passkey, com paciente logado (DPoP + token). O `finish` recebe o header `X-Passkey-Challenge`.
- `POST /action/begin`: desafio para assinar uma ação. Corpo: `{"action": "consent:grant:clinical_history_read"}`. Sem passkey cadastrada, responde 403.
- `POST /recovery/request`: envia código de 6 dígitos para o e-mail (público, sempre responde 202).
- `POST /recovery/verify`: troca o código por `recovery_token` (público, 10 minutos).
- `POST /recovery/register/begin` e `POST /recovery/register/finish`: cadastro da passkey nova usando o header `X-Recovery-Token`.

Conceder (`POST /api/consents`) e revogar (`DELETE /api/consents/:id`) exigem os headers `X-Passkey-Challenge` e `X-Passkey-Assertion` (JSON da assinatura, em base64url). Sem eles, respondem 401.

## Como o app liga

1. Login normal no app (já existia).
2. Na primeira ação de consentimento, o app chama `action/begin`. Se responder 403, cadastra a passkey com `register/begin` e `register/finish`, e repete a ação.
3. Quem perdeu o aparelho usa "Perdi o acesso à passkey" na tela de login.

## Deploy do back

1. Subir a branch `feature/passkey-consentimento` (ou o `main`, quando for mesclada).
2. Definir `WEBAUTHN_RP_ID` e `WEBAUTHN_ORIGINS` no ambiente do servidor.
3. Reiniciar o serviço. As migrações rodam no startup.
4. Conferir: `POST /api/passkeys/recovery/request` com um e-mail qualquer tem que responder 202. Hoje responde 404 no servidor.

## Deploy do app (web)

- `EXPO_PUBLIC_API_BASE_URL=https://api.pohinc.com.br`
- `npx expo export --platform web` gera `dist/`. Publicar `dist/` em `https://app.pohinc.com.br`, com HTTPS.
- Os arquivos `public/.well-known/apple-app-site-association` e `public/.well-known/assetlinks.json` ficam com placeholders. Só preencher quando o app nativo for gerado.
- Instalação das dependências: `npm install --legacy-peer-deps` (o `react-native-passkeys` tem conflito de peer com a versão canary do Expo).

## Pendências

- App nativo (Android e iOS): precisa de build de desenvolvimento ou EAS. O Expo Go não tem o módulo de passkey.
- Hash do certificado Android e Team ID da Apple.
- Desafios WebAuthn ficam em memória: vale para uma instância só.
- Recuperação não tem limite de envio de código.
