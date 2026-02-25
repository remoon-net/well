English | [中文](./README_zh.md)

---

# well net

well-net is an open-source networking tool that helps users establish a **private internal communication network** with others.

## LAN Chat Software: delta chat

I found that using email protocols is better suited for scenarios where participants may go offline at any time. So I chose [delta chat](https://delta.chat/)
, which supports IP-based email formats such as `remoon@[2001:ff::1]`, making it a perfect fit.

However, current open-source mail servers require TLS by default, while TLS is not needed within a well-net network. Therefore, a dedicated mail server needs to be built specifically for this internal communication scenario.

Unfortunately, I don’t have time to work on this right now. If you know any suitable implementation, feel free to recommend it in the issues.

A recommended mail server implementation should meet the following requirements:

1. Support IP-based email addresses
2. Rewrite the `From` email domain into an internal IP format, for example: `remoon@remoon.net` → `remoon@[2001:ff::1]`, to unify identity
3. Support message re-delivery, since other peers may go offline at any time
4. Possibly other features?

# Usage Demo

## User Demo

https://youtu.be/H-iywrYNtmY

## Service Provider Demo

https://youtu.be/D2iu9xNmfR8

# Frontend Interface

After trying various approaches, using a web UI turned out to be the fastest option.

[well-webui](https://github.com/remoon-net/well-webui)

# Todo

- [x] Plugin mechanism. `_hookjs` is not very ideal, but it allows implementing features like “allow anyone to connect”
- [ ] <del>Unified management of owned devices</del> Temporarily not planned; feels easy to implement with external scripts
- [ ] Support socks proxy

# Dual License

If you would like to release your work as closed source, you can purchase a commercial license from me.
(Contributors must sign a CLA permitting me to commercially sell closed-source licenses based on their contributions.)

If your work is open source, you only need to comply with the GPL 3.0 license and make your code available to the software users. GPL 3.0 does not propagate to the server side, so you are free to modify the server implementation as you wish.
