<script setup lang="ts">
import { RouterLink, RouterView } from 'vue-router'
</script>

<template>
  <nav class="top-nav">
    <RouterLink to="/" class="nav-brand">
      <img src="/logo.png" alt="Sea-Trade Lab" class="nav-logo" />
      SeaTradeLab
    </RouterLink>
    <div class="nav-links">
      <RouterLink to="/">Dashboard</RouterLink>
      <RouterLink to="/map">Map</RouterLink>
    </div>
  </nav>
  <RouterView />
</template>

<style scoped>
.top-nav {
  display: flex;
  align-items: center;
  justify-content: space-between;
  background: #1a1a2e;
  padding: 0.5rem 1.5rem;
  border-bottom: 1px solid #333;
}

.nav-brand {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 1.2rem;
  font-weight: 700;
  color: #e0e0e0;
  text-decoration: none;
}

.nav-logo {
  height: 36px;
  mix-blend-mode: screen;
}

.nav-links a {
  color: #aaa;
  text-decoration: none;
  margin-left: 1.5rem;
  font-size: 0.9rem;
  padding: 0.25rem 0;
}

.nav-links a.router-link-exact-active {
  color: #4fc3f7;
  border-bottom: 2px solid #4fc3f7;
}

.nav-links a:hover {
  color: #e0e0e0;
}
</style>
