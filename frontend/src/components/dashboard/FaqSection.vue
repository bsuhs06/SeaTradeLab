<script setup lang="ts">
import { ref } from 'vue'

const openIndex = ref<number | null>(null)

function toggle(i: number) {
  openIndex.value = openIndex.value === i ? null : i
}

const items = [
  {
    q: 'What is AIS and why does it matter?',
    a: `AIS (Automatic Identification System) is a mandatory tracking system for commercial vessels. Ships broadcast their identity, position, speed, course, and destination via VHF radio. The collector pulls this data from the Finnish Digitraffic Maritime API, which covers the Baltic Sea, Gulf of Finland, and surrounding waters, polling every 10 minutes to build a picture of vessel movements.<br><br>Under international law (IMO SOLAS), all ships over 300 gross tons on international voyages must transmit AIS. Turning off AIS is a red flag that often indicates sanctions evasion, illicit transfers, or smuggling.`,
  },
  {
    q: 'How does Ship-to-Ship (STS) transfer detection work?',
    a: `The STS detector scans position data to find pairs of vessels that come within close proximity (default: 500 meters), both slow down significantly (speed under 3 knots), and remain close for a sustained period (15+ minutes). The algorithm uses a KD-tree spatial index for efficient proximity searching, then computes precise geodesic distances for candidate pairs. Events are assigned confidence scores based on duration.`,
  },
  {
    q: 'What are "dark vessels" and AIS gaps?',
    a: `A "dark vessel" is one that has stopped transmitting AIS data. The gap detector tracks this by comparing each vessel's last known position timestamp against the current time. Vessels are flagged as dark when their AIS gap exceeds a configurable threshold (default: 6 hours).<br><br>Important caveat: the Finnish Digitraffic API has regional coverage. A vessel sailing out of the Baltic may appear to "go dark" simply because it left the coverage area.`,
  },
  {
    q: 'How does Russian port visit detection work?',
    a: `The port visit detector monitors vessel positions near key Russian ports: Primorsk, Ust-Luga, Vysotsk, St. Petersburg, and Kaliningrad. Non-Russian vessels visiting these ports are particularly interesting for sanctions monitoring. A visit is recorded when a vessel remains within the port boundary (typically 4-8 km radius) for at least 30 minutes.`,
  },
  {
    q: 'What is a "ghost fleet" / dark fleet?',
    a: `The "dark fleet" or "shadow fleet" refers to a growing network of aging tankers used to circumvent international sanctions on Russian oil exports. These vessels often operate under flags of convenience, turn off AIS transponders during transfers, conduct ship-to-ship transfers in open water, and lack proper insurance. This tracker focuses on detecting the behavioral signatures of these operations.`,
  },
  {
    q: 'Data source and coverage area',
    a: `Data comes from the <strong>Finnish Digitraffic Maritime API</strong>, a free, public API provided by the Finnish Transport Infrastructure Agency. Coverage includes the Gulf of Finland, Baltic Sea, Gulf of Bothnia, and approaches to St. Petersburg, Helsinki, Tallinn, Stockholm. Data is polled every 10 minutes.`,
  },
]
</script>

<template>
  <div class="faq">
    <h2>How it works</h2>
    <div v-for="(item, i) in items" :key="i" class="fi">
      <div class="fq" @click="toggle(i)">
        <span>{{ item.q }}</span>
        <span class="t">{{ openIndex === i ? '-' : '+' }}</span>
      </div>
      <div class="fa" :class="{ open: openIndex === i }" v-html="item.a" />
    </div>
  </div>
</template>

<style scoped>
.faq { margin-top: 32px; }
.faq h2 { font-size: 22px; margin-bottom: 14px; color: #fff; }
.fi { background: rgba(20, 24, 33, 0.85); border-radius: 8px; border: 1px solid rgba(255, 255, 255, 0.07); margin-bottom: 8px; overflow: hidden; }
.fq { padding: 14px 18px; cursor: pointer; font-weight: 600; display: flex; justify-content: space-between; align-items: center; font-size: 14px; }
.fq:hover { background: rgba(255, 255, 255, 0.03); }
.t { color: #7cb4ff; font-size: 18px; }
.fa { padding: 0 18px; font-size: 13px; line-height: 1.8; color: #bbb; max-height: 0; overflow: hidden; transition: max-height 0.3s ease, padding 0.3s ease; }
.fa.open { max-height: 600px; padding: 0 18px 16px; }
</style>
