<!--
  SySceneForm — modal that builds a SceneConfig and writes the
  regenerated Pkl to scenes/<id>.pkl via EditSessionService.
  Mirrors SyAutomationForm but with the scene shape (id, displayName,
  areaId, actions[]).
-->
<script setup lang="ts">
import { ref, watch } from "vue";
import { SySheet, SyText, SyButton, SyInput, SyIcon } from "@/lib";
import ActionEditor, { type ActionValue } from "@/views/automations/ActionEditor.vue";
import { regenPreview } from "@/data/regen-preview";
import { openForEdit, commitEdit } from "@/data/edit-session";

const props = defineProps<{
  open: boolean;
  /** Pre-set areaId. Empty = global. */
  areaId: string;
  /** Optional prefill for edit. */
  initial?: {
    id: string;
    displayName?: string;
    actions: ActionValue[];
  };
}>();

const emit = defineEmits<{
  (e: "update:open", v: boolean): void;
  (e: "saved", id: string): void;
}>();

const id = ref<string>("");
const displayName = ref<string>("");
const actions = ref<ActionValue[]>([]);
const saveBusy = ref<boolean>(false);
const saveError = ref<string>("");

function reset(): void {
  if (props.initial) {
    id.value = props.initial.id;
    displayName.value = props.initial.displayName ?? "";
    actions.value = props.initial.actions;
  } else {
    id.value = "";
    displayName.value = "";
    actions.value = [];
  }
  saveError.value = "";
}

watch(() => props.open, (o) => { if (o) reset(); });

function close(): void { emit("update:open", false); }
function addAction(): void { actions.value = [...actions.value, { kind: "call_service" }]; }

function actionToProto(a: ActionValue): Record<string, unknown> {
  if (a.kind === "call_service") {
    return {
      callService: {
        entity: a.entity ?? "",
        capability: a.capability ?? "",
        args: a.args ?? {},
      },
    };
  }
  return {};
}

function buildAst(): Record<string, unknown> {
  return {
    id: id.value,
    displayName: displayName.value,
    areaId: props.areaId,
    actions: actions.value.map(actionToProto),
  };
}

async function save(): Promise<void> {
  if (!id.value) {
    saveError.value = "id is required";
    return;
  }
  saveBusy.value = true;
  saveError.value = "";
  try {
    const ast = buildAst();
    const { pklText } = await regenPreview({ fileType: "scene", astJson: JSON.stringify(ast) });
    const filePath = `scenes/${id.value}.pkl`;
    const session = await openForEdit(filePath);
    const r = await commitEdit({
      filePath,
      lockToken: session.lockToken,
      regeneratedPkl: pklText,
      expectedFileHash: session.fileHash,
    });
    if (r.conflict) {
      saveError.value = `Conflict: ${r.conflict.reason}`;
      return;
    }
    emit("saved", id.value);
    close();
  } catch (err) {
    saveError.value = err instanceof Error ? err.message : String(err);
  } finally {
    saveBusy.value = false;
  }
}
</script>

<template>
  <SySheet :model-value="open" side="right" size="lg" title="Scene" @update:model-value="(v: boolean) => emit('update:open', v)">
    <div class="form">
      <section class="form__section">
        <SyText variant="label" tone="subtle">Identity</SyText>
        <SyInput :model-value="id" placeholder="id (e.g. movie-night)" @update:model-value="(s: string) => id = s" />
        <SyInput :model-value="displayName" placeholder="displayName" @update:model-value="(s: string) => displayName = s" />
        <SyText v-if="areaId" variant="caption" tone="subtle">Scoped to room: {{ areaId }}</SyText>
        <SyText v-else variant="caption" tone="subtle">Global scene (no room scope)</SyText>
      </section>

      <section class="form__section">
        <div class="form__sectionHead">
          <SyText variant="label" tone="subtle">Actions</SyText>
          <SyButton intent="ghost" size="sm" @click="addAction"><SyIcon name="plus" :size="12" /> Add</SyButton>
        </div>
        <ActionEditor
          v-for="(a, i) in actions" :key="i"
          :model-value="a"
          @update:model-value="(v: ActionValue) => actions[i] = v"
          @remove="actions = actions.filter((_, j) => j !== i)"
        />
      </section>

      <SyText v-if="saveError" variant="caption" tone="bad">{{ saveError }}</SyText>

      <footer class="form__foot">
        <SyButton intent="ghost" @click="close" :disabled="saveBusy">Cancel</SyButton>
        <SyButton intent="primary" :disabled="saveBusy || !id" @click="save">
          {{ saveBusy ? "Saving…" : "Save" }}
        </SyButton>
      </footer>
    </div>
  </SySheet>
</template>

<style scoped>
.form { display: flex; flex-direction: column; gap: var(--sy-space-4); padding: var(--sy-space-3); }
.form__section { display: flex; flex-direction: column; gap: var(--sy-space-2); }
.form__sectionHead { display: flex; align-items: center; justify-content: space-between; }
.form__foot { display: flex; gap: var(--sy-space-2); justify-content: flex-end; padding-top: var(--sy-space-3); border-top: 1px solid var(--sy-color-line-soft); }
</style>
