<!--
  AccountSection — shows the signed-in user and offers sign-out.

  Today this is informational only. Once AuthService is wired we'll
  call AuthService/WhoAmI on mount and SignOut from the button.
-->
<script setup lang="ts">
import { SyText, SySurface, SyButton, SyAvatar, SyBadge } from "@/lib";

/* TODO: pull from AuthService/WhoAmI rather than hardcoding. The local
   peercred auth grants `system:local` to the calling OS user, but
   user-facing names/emails aren't surfaced over RPC yet. */
const user = {
  name: "Fynn Datoo",
  email: "fdatoo7@gmail.com",
  principal: "system:local",
} as const;

function onSignOut(): void {
  /* TODO: AuthService/SignOut + redirect. */
}
</script>

<template>
  <section class="section">
    <header class="section__head">
      <SyText as="h1" variant="display">Account</SyText>
      <SyText variant="body" tone="subtle">
        Your identity inside Switchyard.
      </SyText>
    </header>

    <SySurface>
      <div class="section__user">
        <SyAvatar :name="user.name" size="lg" />
        <div class="section__userText">
          <SyText weight="medium">{{ user.name }}</SyText>
          <SyText variant="caption" tone="subtle">{{ user.email }}</SyText>
        </div>
      </div>

      <dl class="section__facts">
        <div class="section__fact">
          <dt><SyText variant="caption" tone="subtle">Principal</SyText></dt>
          <dd>
            <SyBadge intent="neutral">{{ user.principal }}</SyBadge>
          </dd>
        </div>
        <div class="section__fact">
          <dt><SyText variant="caption" tone="subtle">Session</SyText></dt>
          <dd>
            <SyText variant="caption">
              Authenticated via local socket peer credentials.
            </SyText>
          </dd>
        </div>
      </dl>
    </SySurface>

    <div class="section__footer">
      <SyButton intent="danger" @click="onSignOut">Sign out</SyButton>
    </div>
  </section>
</template>

<style scoped>
.section {
  display: flex;
  flex-direction: column;
  gap: var(--sy-space-5);
}
.section__head {
  display: flex;
  flex-direction: column;
  gap: var(--sy-space-1);
}
.section__user {
  display: flex;
  align-items: center;
  gap: var(--sy-space-3);
  padding-bottom: var(--sy-space-3);
  border-bottom: 1px solid var(--sy-color-line-soft);
  margin-bottom: var(--sy-space-3);
}
.section__userText {
  display: flex;
  flex-direction: column;
  gap: var(--sy-space-0);
}
.section__facts {
  display: flex;
  flex-direction: column;
  gap: var(--sy-space-3);
  margin: 0;
}
.section__fact {
  display: grid;
  grid-template-columns: 140px 1fr;
  align-items: center;
  gap: var(--sy-space-3);
}
.section__fact dt, .section__fact dd { margin: 0; }
.section__footer {
  display: flex;
  justify-content: flex-end;
}
</style>
