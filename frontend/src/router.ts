import { createRouter, createWebHistory } from 'vue-router'

const Home = () => import('./views/Home.vue')
const Admin = () => import('./views/Admin.vue')
const Share = () => import('./views/Share.vue')
const RecipientPage = () => import('./views/RecipientPage.vue')
const MuteNotificationsPage = () => import('./views/MuteNotificationsPage.vue')

declare module 'vue-router' {
  interface RouteMeta {
    // Routes exempt from the LoginGate check in App.vue — for pages an
    // external recipient with no BCC session must be able to reach.
    public?: boolean
  }
}

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'home', component: Home },
    { path: '/admin', name: 'admin', component: Admin },
    { path: '/share', name: 'share', component: Share },
    // The surface was called Send until it was renamed. Mail already sent
    // links here with ?tab=sent&package=..., so the query has to survive the
    // redirect — without it the author lands on the compose form instead of
    // the package they were mailed about.
    { path: '/send', redirect: (to) => ({ path: '/share', query: to.query }) },
    {
      path: '/s/:packageId',
      name: 'recipient',
      component: RecipientPage,
      meta: { public: true },
    },
    {
      path: '/mute/:token',
      name: 'mute',
      component: MuteNotificationsPage,
      meta: { public: true },
    },
  ],
})
