import PocketBase, {
  RecordAuthResponse,
  RecordModel,
} from "npm:pocketbase@0.26.8"

const pb = new PocketBase("http://127.0.0.1:7799")
pb.beforeSend = (url, options) => {
  console.log("执行请求", options.method, url)
  return { url, options }
}

const auth = {
  user: "well@remoon.net",
  pass: "well@remoon.net",
}

import { jwtDecode } from "npm:jwt-decode@4.0.0"
// @ts-types="npm:@types/luxon@3.7.1"
import { DateTime } from "npm:luxon@3.7.2"

async function login(su?: RecordAuthResponse<RecordModel>) {
  if (su) {
    const token = jwtDecode(su.token)
    const t = DateTime.fromSeconds(token.exp!)
    const w = t.diffNow("seconds").seconds - 60
    await new Promise((rl) => setTimeout(rl, w * 1e3))
  }
  return pb.collection("_superusers").authWithPassword(auth.user, auth.pass)
}

const su = await login()
async function loginKeep(su: RecordAuthResponse<RecordModel>) {
  while (true) {
    su = await login(su)
  }
}
loginKeep(su)

interface Peer extends RecordModel {
  disabled: boolean
  ipv6: string
}

const peers = pb.collection<Peer>("peers")
const f = pb.filter(`name = '' && disabled = true && ipv6 = ''`)
await peers.subscribe(
  "*",
  (d) => {
    if (d.action !== "create") {
      return
    }
    peers.update(d.record.id, { disabled: false, ipv6: "auto" })
  },
  {
    filter: f,
  },
)

async function fixLost() {
  while (true) {
    await new Promise((rl) => setTimeout(rl, 60e3))
    const peerList = await peers.getFullList({
      filter: f,
    })
    for (const p of peerList) {
      await peers.update(p.id, { disabled: false, ipv6: "auto" })
    }
  }
}
fixLost()
