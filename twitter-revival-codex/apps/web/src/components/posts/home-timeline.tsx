"use client";
import Link from "next/link";
import { Icon } from "@/components/layout/icon";
import { PostComposer } from "./composer";
import { PostList } from "./post-list";
export function HomeTimeline() {
  return (
    <>
      <header className="feed-header">
        <div>
          <h1>Home</h1>
          <p>Le voci che hai scelto di seguire</p>
        </div>
        <span className="header-symbol">
          <Icon name="spark" />
        </span>
      </header>
      <div className="feed-tab">
        <strong>Seguiti · In ordine cronologico</strong>
      </div>
      <div id="compose">
        <PostComposer />
      </div>
      <div className="demo-notice" role="note">
        <span className="demo-notice-icon">
          <Icon name="leaf" size={19} />
        </span>
        <div>
          <strong>La conversazione comincia qui.</strong>
          <p>
            Post tuoi e degli account che segui, inclusi i repost. Nessun
            contenuto demo in questa timeline.{" "}
            <Link href="/people" className="text-button">
              Trova persone per handle
            </Link>
          </p>
        </div>
      </div>
      <PostList
        path="/feed/home"
        empty="Qui c’è spazio per il primo pensiero. Pubblica un post o segui qualcuno per iniziare."
        label="Timeline dei seguiti"
      />
    </>
  );
}
