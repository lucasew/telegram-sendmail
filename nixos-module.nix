{ lib, pkgs, config, ... }:
let
  inherit (lib) mkEnableOption mkIf mkOption types;
  cfg = config.services.telegram-sendmail;
in
{
  options = {
    services.telegram-sendmail = {
      enable = mkEnableOption "telegram-sendmail";
      credentialFile = mkOption {
        description = "Dotenv file used in the service. Should not be a nix-store path.";
        type = types.path;
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
      after = [ "network-online.target" ];
      unitConfig = {
        StartLimitIntervalSec = 0;
      };
      serviceConfig = {
        Restart = "always";
        RestartSec = 1;
        RuntimeDirectory = "telegram-sendmail";
        EnvironmentFile = [ cfg.credentialFile ];
        User = "telegram_sendmail";
      };
      script = let
        telegram_mail = pkgs.stdenvNoCC.mkDerivation {
          name = "telegram_mail";
          dontUnpack = true;
          preferLocalBuild = true;
          buildInputs = with pkgs; [ python3 ];
          installPhase = ''
                install -m 555 ${./service} $out
                patchShebangs $out
          '';
        };
      in ''
        socket="$RUNTIME_DIRECTORY/socket.sock"
        echo Listening on $socket
        ${telegram_mail} -b "$socket" -n "${config.networking.hostName}"
      '';
    };

    services.mail.sendmailSetuidWrapper = {
      program = "sendmail";
      source = pkgs.writeShellScript "sendmail" ''
        ${pkgs.netcat}/bin/nc -N -U /run/telegram-sendmail/socket.sock
      '';
      setuid = false;
      setgid = false;
      owner = "root";
      group = "root";
    };
  };
}
