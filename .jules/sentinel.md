## 2023-10-27 - Restrict UNIX Socket Permissions
**Vulnerability:** The UNIX socket was being created with `0o777` permissions, making it world-writable. This allowed any user on the system to send messages through the socket.
**Learning:** Leaving file and socket permissions open to everyone is a significant security risk. It's a common oversight, especially in services that need to be accessible by other processes. The principle of least privilege should always be applied.
**Prevention:** Always define the most restrictive permissions possible for any file, socket, or resource. In this case, changing the permissions to `0o770` ensures that only the owner and members of the group can access the socket, which is the intended behavior for a service like this.
