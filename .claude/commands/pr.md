---
description: Creates a PR using the backend PR template and conventional commits format
---

Crea un Pull Request siguiendo estos pasos.

## Paso 1.5: Actualizar la rama con main

Antes de analizar los cambios, asegúrate de que la rama esté actualizada con `main`.

1. Guarda la rama actual:


git branch --show-current


2. Actualiza referencias remotas:


git fetch origin


3. Cambia a main:


git checkout main


4. Actualiza main:


git pull origin main


5. Regresa a la rama original:


git checkout <branch>


6. Haz merge de main:


git merge main


### Si hay conflictos

- Detente inmediatamente
- Informa al usuario que hay conflictos
- Pide que los resuelva antes de continuar

## Paso 1: Analizar los cambios

1. Obtén la rama actual:


git branch --show-current


2. Extrae el número de ticket del nombre de la rama.

Ejemplo:


feat/123-add-payroll-endpoint
→ ticket = 123


3. Verifica que no haya cambios sin commitear:


git status --short


Si hay cambios pendientes, informa al usuario y **detente**.

4. Verifica que la rama tenga upstream:


git rev-parse --abbrev-ref --symbolic-full-name @{u}


Si no existe upstream:


git push -u origin HEAD


5. Analiza los cambios contra main:


git diff main...HEAD


y revisa commits:


git log main..HEAD --oneline


---

# Paso 2: Generar el título del PR

El título debe seguir el formato:


type(scope): description


Ejemplo:


feat(payroll): New endpoint GET for listing employees payroll with pagination


## Reglas

### type permitido

- feat
- fix
- refactor
- test
- chore
- docs

### scope

El scope debe representar el módulo afectado.

Ejemplos:


auth
employees
payroll
projects
directory
notifications


Si el módulo no es claro, dedúcelo desde:

- archivos modificados
- nombres de servicios
- controllers
- aggregates

### description

La descripción debe:

- ser corta
- empezar con mayúscula
- explicar el cambio principal

Ejemplos:


feat(payroll): Add endpoint to list employee payrolls
fix(auth): Handle expired refresh tokens
refactor(directory): Move validation logic into aggregate


---

# Paso 3: Generar descripción del PR

La descripción debe incluir:

## Description

Explicación breve del propósito del PR.

## Changes Included

Lista de máximo **5 bullet points** que resuman los cambios reales.

Ejemplo:


Add GET /payroll endpoint with pagination

Implement payroll listing service

Add validation for pagination params

Add unit tests for payroll service


---

# Paso 4: Crear el PR

Usa `gh pr create` con este template:


gh pr create --title "título generado" --body "$(cat <<'EOF'

Description

{descripción generada}

Changes Included

{lista de cambios}

Checklist

 Tests added or updated

 Code follows project architecture

Related Issues

Closes #{ticket}

Screenshots (optional)

EOF
)"


El PR siempre debe ir contra **main**.

---

# Paso 5: Confirmar

Después de ejecutar el comando:

1. Mostrar el link generado por `gh`
2. Confirmar que el PR fue creado correctamente.

---

# Notas importantes

- No generar más de **5 bullet points**
- El título debe seguir estrictamente el formato `type(scope): description`
- Si ocurre cualquier error durante el proceso, detenerse y preguntar al usuario