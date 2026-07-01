---
description: Create logical commits following conventional commits and project module scopes
---

Commitea los cambios del worktree agrupándolos por **unidad lógica**, no por archivo.

## Paso 1: Verificar cambios

Ejecuta:


git status --short


Si no hay cambios, informa al usuario y detente.

---

## Paso 2: Analizar cambios

Analiza el diff completo:


git diff


y detecta:

- módulos afectados (`src/contexts/<module>`)
- tests (`*.spec.ts`)
- seeds (`prisma/seed`)
- config (`.gitignore`, config files)

Agrupa los archivos por **unidad lógica**.

---

## Paso 3: Generar commits

Para cada grupo genera un commit siguiendo:


type(scope): description


### Tipos permitidos


feat
fix
refactor
test
chore
docs


### Scope

Derivado del módulo:


src/contexts/compliance → compliance
src/contexts/auth → auth
src/contexts/directory → directory


Ejemplos:


feat(compliance): Add onboarding compliance document flow
test(compliance): Add service tests for compliance onboarding
chore(seed): Add compliance categories seed data
refactor(directory): Move invitation logic into domain event


---

## Paso 4: Crear commits

Para cada grupo:


git add <archivos>
git commit -m "mensaje generado"


**Nunca agregar**


Co-Authored-By


---

## Paso 5: Confirmar

Mostrar:


git log --oneline -10


---

## Reglas importantes

- No crear commits por archivo
- Agrupar cambios relacionados
- Máximo **5 commits**
- Usar siempre `type(scope): description`
- Si hay archivos sensibles (`.env`, secretos), advertir antes de continuar