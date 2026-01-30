{ lib, pkgs, config, ... }:
let
  inherit (lib) mkEnableOption mkIf mkOption types;
  cfg = config.services.telegram-sendmail;
  socketPath = "/run/telegram-sendmail/socket.sock";
  serviceName = "telegram-sendmail";

  telegram-sendmail-pkg = pkgs.buildGoModule {
    pname = "telegram-sendmail";
    version = "0.0.1";
    src = ./.;
    vendorHash = null;
  };
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

    systemd.sockets.telegram-sendmail = {
      description = "Telegram Sendmail Socket";
      wantedBy = [ "sockets.target" ];
      listenStreams = [ socketPath ];
      socketConfig = {
        SocketMode = "0777";
      };
    };

    systemd.services.telegram-sendmail = {
      description = "Telegram Sendmail Service";
      requires = [ "telegram-sendmail.socket" ];
      after = [ "network.target" ];

      serviceConfig = {
        RuntimeDirectory = serviceName;
        StateDirectory = serviceName;
        Restart = "on-failure";
        RestartSec = 1;
        EnvironmentFile = [ cfg.credentialFile ];
        User = "telegram_sendmail";
        Group = "telegram_sendmail";
        ExecStart = "${telegram-sendmail-pkg}/bin/telegram-sendmail serve ${lib.escapeShellArgs cfg.extraArgs}";
      };
    };

    services.mail.sendmailSetuidWrapper = {
      program = "sendmail";
      source = pkgs.writeShellScript "sendmail" ''
        # Check for socket availability with timeout
        for i in $(seq 1 30); do
          if [ -S "${socketPath}" ]; then
            ${pkgs.netcat}/bin/nc -N -U "${socketPath}"
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
