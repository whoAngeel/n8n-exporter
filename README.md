# n8n Workflow Exporter

CLI en Go para exportar workflows de n8n de forma interactiva, limpiarlos automáticamente y dejarlos listos para importar en otra instancia.

## ¿Qué hace?

Conecta a tu instancia n8n vía API REST, muestra todos tus workflows en una interfaz interactiva de terminal (resaltando los editados hoy) y exporta los que selecciones como archivos `.json` limpios — idénticos a los que produce el botón "Download" de n8n, sin metadatos de la instancia origen.

### Limpieza automática

Cada workflow exportado conserva solo los campos que n8n necesita para importar:

| Conservado | Eliminado |
|---|---|
| `name` | `id` |
| `nodes` | `versionId` |
| `connections` | `active` |
| `settings: {}` | `tags`, `meta`, `pinData` |
| | `createdAt`, `updatedAt` |
| | `activeVersion`, `isArchived` |

`settings` siempre se exporta como `{}` para evitar que configuraciones específicas de la instancia origen (como `binaryDataMode`) interfieran en el destino.

## Compilar y ejecutar

```bash
go build -o n8n-exporter .

# Ejecutar
./n8n-exporter        # macOS / Linux
n8n-exporter.exe      # Windows
```

## Uso

```bash
n8n-exporter
```

### Primera vez

El CLI te pedirá:

```
n8n instance URL: https://tu-instancia.com
API Token: ••••••••••••••••

Output directory for exports
(leave empty to use current directory): C:\mis-workflows

Save credentials for future sessions? (y/N): y
Choose a passphrase: ••••••••
✓ Credentials saved.
```

### Siguientes veces

Solo necesitas la passphrase:

```
Passphrase: ••••••••
🔗 Connecting to https://tu-instancia.com...
✓ Found 43 workflow(s)
```

### Selección interactiva

```
  n8n Workflow Exporter  [INCLUSIÓN]
  ───────────────────────────────────────────────────────
  ↑/↓ navegar   SPACE marcar   TAB cambiar modo   ENTER exportar   q salir

  ▶ [ ]  a.act
    [✓]  sw.ExtractText
    [ ]  xc.Final.Declaration
    ...

  Workflows a exportar: 1 / 43
```

| Tecla | Acción |
|---|---|
| `↑` / `↓` o `k` / `j` | Navegar |
| `SPACE` | Marcar / desmarcar |
| `TAB` | Cambiar entre modo inclusión y exclusión |
| `ENTER` | Confirmar y exportar |
| `q` / `Ctrl+C` | Cancelar |

**Modo inclusión** — exporta solo los marcados.  
**Modo exclusión** — exporta todos menos los marcados.

### Resultado

```
📦 Exporting 3 workflow(s)...

  ✓ sw.ExtractText                          → sw.ExtractText.json
  ✓ xc.Final.Declaration                   → xc.Final.Declaration.json
  ✓ sw.assessment.upsert                   → sw.assessment.upsert.json

✓ Done: 3 exported, 0 errors.
📁 Output: C:\mis-workflows
```

## Credenciales guardadas

Las credenciales (URL, token/contraseña, directorio de exportación) se guardan encriptadas en:

| Sistema | Ruta                                                         |
| ---------| --------------------------------------------------------------|
| Windows | `%AppData%\n8n-exporter\credentials.enc`                     |
| macOS   | `~/Library/Application Support/n8n-exporter/credentials.enc` |
| Linux   | `~/.config/n8n-exporter/credentials.enc`                     |

La encriptación usa **AES-256-GCM** con clave derivada de tu passphrase mediante **PBKDF2-SHA256** (100,000 iteraciones). Sin la passphrase el archivo es ilegible.

### Resetear credenciales

```bash
n8n-exporter --reset
```

La próxima ejecución pedirá todas las credenciales de nuevo.

## Autenticación n8n

El CLI requiere un **API Token** para conectar con tu instancia de n8n. 
Puedes crearlo en n8n yendo a `Settings` → `API` → `Create an API key`.

> **Nota:** El login con usuario y contraseña (Basic Auth) ya no está soportado.

## Requisitos

- Go 1.21+
- Instancia n8n con API habilitada

## Dependencias

| Paquete | Uso |
|---|---|
| `github.com/charmbracelet/bubbletea` | Motor TUI |
| `github.com/charmbracelet/lipgloss` | Estilos de terminal |
| `golang.org/x/crypto` | PBKDF2 para derivación de clave |
| `golang.org/x/term` | Lectura de inputs sin eco |
