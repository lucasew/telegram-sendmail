{ lib, pkgs, config, ... }:
let
  inherit (lib) mkEnableOption mkIf mkOption types;
  cfg = config.services.telegram-sendmail;
  socketPath = "/run/telegram-sendmail/socket.sock";
  serviceName = "telegram-sendmail";

  src = ./.;
  telegram-sendmail-pkg = pkgs.buildGoModule {
    pname = "telegram-sendmail";
    version = src.rev or "dirty";
    inherit src;
    vendorHash = "sha256-ofMGVrFz9SofDITDr4JBUCuT0Lpd1YDXamKwowUgVuI=";
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
        description = "Extra CLI flags for `telegram-sendmail serve` (see --help).";
        type = types.listOf types.str;
        default = [];
        # Prefer flags that exist on `telegram-sendmail serve` (see --help).
        example = [ "--hostname=mailhost" "--socket-timeout=30" ];
      };
    };
  };

  config = mkIf cfg.enable {
    systemd.sockets.telegram-sendmail = {
      description = "Telegram Sendmail Socket";
      wantedBy = [ "sockets.target" ];
      listenStreams = [ socketPath ];
      socketConfig = {
        # World-traversable parent: sendmail clients run as any user.
        # Do not set RuntimeDirectory=telegram-sendmail on the service
        # (DynamicUser would privatize /run/telegram-sendmail).
        DirectoryMode = "0755";
        SocketMode = "0777";
      };
    };

    systemd.services.telegram-sendmail = {
      description = "Telegram Sendmail Service";
      requires = [ "telegram-sendmail.socket" ];
      # After=socket: Requires= alone starts units in parallel (Listeners race).
      after = [ "network.target" "telegram-sendmail.socket" ];

      serviceConfig = {
        DynamicUser = true;
        StateDirectory = serviceName;
        Restart = "on-failure";
        RestartSec = 1;
        EnvironmentFile = [ cfg.credentialFile ];
        ExecStart = "${telegram-sendmail-pkg}/bin/telegram-sendmail serve ${lib.escapeShellArgs cfg.extraArgs}";
      };
    };

    services.mail.sendmailSetuidWrapper = {
      program = "sendmail";
      source = pkgs.writeShellScript "sendmail" ''
        exec ${telegram-sendmail-pkg}/bin/telegram-sendmail sendmail "$@"
      '';
      setuid = false;
      setgid = false;
      owner = "root";
      group = "root";
    };
  };
}
