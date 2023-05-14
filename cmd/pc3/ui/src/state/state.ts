import { reactive } from 'vue'

export const store: {
    targets: string[] | 'all'
} = reactive({
  targets: []
})