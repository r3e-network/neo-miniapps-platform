import { BrowserRouter, Routes, Route } from 'react-router-dom';
import { Header } from './components/Header';
import { HomePage } from './pages/HomePage';
import { BuyTicketPage } from './pages/BuyTicketPage';
import { MyTicketsPage } from './pages/MyTicketsPage';
import { ResultsPage } from './pages/ResultsPage';
import { HowToPlayPage } from './pages/HowToPlayPage';

function App() {
  return (
    <BrowserRouter>
      <div className="min-h-screen">
        <Header />
        <main className="container mx-auto px-4 py-8">
          <Routes>
            <Route path="/" element={<HomePage />} />
            <Route path="/buy" element={<BuyTicketPage />} />
            <Route path="/tickets" element={<MyTicketsPage />} />
            <Route path="/results" element={<ResultsPage />} />
            <Route path="/how-to-play" element={<HowToPlayPage />} />
          </Routes>
        </main>
        <footer className="border-t border-gray-800 py-6 mt-12">
          <div className="container mx-auto px-4 text-center text-gray-500 text-sm">
            <p>MegaLottery - Powered by Neo N3 &amp; Service Layer VRF</p>
            <p className="mt-1">Provably fair draws with verifiable random numbers</p>
          </div>
        </footer>
      </div>
    </BrowserRouter>
  );
}

export default App;
