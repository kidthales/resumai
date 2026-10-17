<script lang="ts">
    import SettingsModal from './components/SettingsModal.svelte';
    import SplitPane from './components/SplitPane.svelte';
    import TopBar from './components/TopBar.svelte';
</script>

<div class="app-shell">
    <TopBar />
    <SplitPane />
</div>

<SettingsModal />
