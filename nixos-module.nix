{ lib, pkgs, config, ... }:
let
  inherit (lib) mkEnableOption mkIf mkOption types;
  cfg = config.services.telegram-sendmail;
  socket = "/run/telegram-sendmail/socket.sock";
  serviceName = "telegram-sendmail";
in
{
  options = {
    services.telegram-sendmail = {
      enable = mkEnableOption "telegram-sendmail service";
      credentialFile = mkOption {
        description = "Dotenv file used in the service. Should not be a nix-store path.";
        type = types.path;
        example = "/path/to/credentials.env";
      };
      extraArgs = mkOption {
        description = "Extra arguments to pass to the script";
        type = types.listOf types.str;
        default = [];
        example = [ "--verbose" "--debug" ];
      };
    };
  };

  config = mkIf cfg.enable {
    users.users.telegram_sendmail = {
      isSystemUser = true;
      group = "telegram_sendmail";
      description = "telegram-sendmail service user";
    };
    users.groups.telegram_sendmail = {};

    systemd.services.telegram-sendmail = {
      description = "Telegram Sendmail Service";
      wantedBy = [ "multi-user.target" ];
      unitConfig = {
        StartLimitIntervalSec = 0;
      };
      serviceConfig = {
        RuntimeDirectory = serviceName;
        StateDirectory = serviceName;
        Restart = "always";
        RestartSec = 1;
        EnvironmentFile = [ cfg.credentialFile ];
        User = "telegram_sendmail";
      };
      script = let
        telegram_mail = pkgs.stdenvNoCC.mkDerivation {
          name = "telegram_mail";
          dontUnpack = true;
          preferLocalBuild = true;
          allowSubstitutes = false;
          buildInputs = with pkgs; [ python3 ];
          installPhase = ''
            install -m 555 ${./service} $out
            patchShebangs $out
          '';
        };
      in ''
        ${telegram_mail} ${lib.escapeShellArgs ([
          "-b" "${socket}"
          "-n" "${config.networking.hostName}"
        ] ++ cfg.extraArgs)}
      '';
    };

    services.mail.sendmailSetuidWrapper = {
      program = "sendmail";
      source = pkgs.writeShellScript "sendmail" ''
        # Check for socket availability with timeout
        for i in $(seq 1 30); do
          if [ -S "${socket}" ]; then
            ${pkgs.netcat}/bin/nc -N -U "${socket}"
            exit $?
          fi
          echo "Waiting for the sendmail socket to be available... (attempt $i/30)" >&2
          sleep 1
        done
        echo "Error: Socket not available after 30 seconds" >&2
        exit 1
      '';
      setuid = false;
      setgid = false;
      owner = "root";
      group = "root";
    };
  };
}
