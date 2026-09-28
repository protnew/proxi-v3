; NSIS hooks — Tauri v2 "include" file.
; P-C: user-data backup MUST run before the old service is stopped/upgraded,
; so a failed install never leaves the user with stopped service + no copy.

!define PROXI_SERVICE_NAME "proxi04-vpn-helper"
!define PROXI_DATA_DIR "$%ProgramData%\\Proxi"

!macro NSIS_HOOK_PREINSTALL
  ; 1) Backup DB+keys first (before touching the service).
  IfFileExists "${PROXI_DATA_DIR}\messenger.db" 0 proxi_backup_done
    CreateDirectory "${PROXI_DATA_DIR}\backups"
    GetTime /windows /utc
    Pop $0
    CopyFiles /SILENT "${PROXI_DATA_DIR}\messenger.db" "${PROXI_DATA_DIR}\backups\messenger-preinstall-$0.db"
    CopyFiles /SILENT "${PROXI_DATA_DIR}\messenger.db-wal" "${PROXI_DATA_DIR}\backups\messenger-preinstall-$0.db-wal"
    CopyFiles /SILENT "${PROXI_DATA_DIR}\messenger.db-shm" "${PROXI_DATA_DIR}\backups\messenger-preinstall-$0.db-shm"
  proxi_backup_done:

  ; 2) Only then stop the service.
  nsExec::ExecToLog 'sc stop "${PROXI_SERVICE_NAME}"'
  Sleep 2000
!macroend

!macro NSIS_HOOK_POSTINSTALL
  ; Start helper service back after files are in place.
  nsExec::ExecToLog 'sc start "${PROXI_SERVICE_NAME}"'
!macroend
