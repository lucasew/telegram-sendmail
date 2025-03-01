{ lib, pkgs, config, ... }:
let
  inherit (lib) mkEnableOption mkIf mkOption types;
  cfg = config.services.telegram-sendmail;
  socket = "/run/telegram-sendmail/socket.sock";
in
{
  options = {
    services.telegram-sendmail = {
      enable = mkEnableOption "telegram-sendmail";
      credentialFile = mkOption {
        description = "Dotenv file used in the service. Should not be a nix-store path.";
        type = types.path;
      };
      extraArgs = mkOption {
        description = "Extra arguments to pass to the script";
        type = types.listOf types.str;
      };
    };
  };

  config = mkIf cfg.enable {
    users.users.telegram_sendmail = {
      isSystemUser = true;
      group = "telegram_sendmail";
    };
    users.groups.telegram_sendmail = {};

    systemd.services.telegram-sendmail = {
      wantedBy = [ "multi-user.target" ];
      unitConfig = {
        StartLimitIntervalSec = 0;
      };
      serviceConfig = {
        RuntimeDirectory = "telegram-sendmail";
        StateDirectory = "telegram-sendmail";
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
        while [ ! -S "${socket}" ]; do
          echo Waiting for the sendmail socket to be available... >&2
          sleep 1
        done
        ${pkgs.netcat}/bin/nc -N -U "${socket}"
      '';
      setuid = false;
      setgid = false;
      owner = "root";
      group = "root";
    };
  };
}
