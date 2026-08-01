import { ref, watch, type Ref } from 'vue';

export type DisplayMode = 'decoded' | 'carrier';

const displayMode = ref<DisplayMode>('decoded');
const savedPhrase = ref('');
let initialized = false;

function readSessionValue(key: string): string {
  if (typeof window === 'undefined') return '';
  return window.sessionStorage.getItem(key) ?? '';
}

function initialize() {
  if (initialized) return;
  initialized = true;

  const storedMode = readSessionValue('silent-outposts:display-mode');
  displayMode.value = storedMode === 'carrier' ? 'carrier' : 'decoded';
  savedPhrase.value = readSessionValue('silent-outposts:secret-phrase');

  watch(displayMode, (value) => {
    window.sessionStorage.setItem('silent-outposts:display-mode', value);
  });
}

export function useSessionPreferences(): {
  displayMode: Ref<DisplayMode>;
  savedPhrase: Ref<string>;
  savePhrase: (value: string) => void;
  clearPhrase: () => void;
} {
  initialize();

  function savePhrase(value: string) {
    savedPhrase.value = value;
    window.sessionStorage.setItem('silent-outposts:secret-phrase', value);
  }

  function clearPhrase() {
    savedPhrase.value = '';
    window.sessionStorage.removeItem('silent-outposts:secret-phrase');
  }

  return { displayMode, savedPhrase, savePhrase, clearPhrase };
}
