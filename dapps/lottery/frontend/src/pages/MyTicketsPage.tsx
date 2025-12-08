import { useState } from 'react';
import { Ticket, Wallet, Filter } from 'lucide-react';
import { TicketCard } from '../components/TicketCard';
import { useUserTickets, useRecentDraws } from '../hooks/useLottery';
import { useWallet } from '../hooks/useWallet';

type FilterType = 'all' | 'pending' | 'won' | 'lost';

export function MyTicketsPage() {
  const { address } = useWallet();
  const { data: tickets, isLoading } = useUserTickets();
  const { data: draws } = useRecentDraws(50);
  const [filter, setFilter] = useState<FilterType>('all');

  // Create a map of draw results
  const drawMap = new Map(draws?.map((d) => [d.drawId, d]) || []);

  // Filter tickets
  const filteredTickets = tickets?.filter((ticket) => {
    const draw = drawMap.get(ticket.drawId);

    switch (filter) {
      case 'pending':
        return !draw?.completed;
      case 'won':
        return ticket.prizeTier > 0;
      case 'lost':
        return draw?.completed && ticket.prizeTier === 0;
      default:
        return true;
    }
  });

  // Stats
  const stats = {
    total: tickets?.length || 0,
    pending: tickets?.filter((t) => !drawMap.get(t.drawId)?.completed).length || 0,
    won: tickets?.filter((t) => t.prizeTier > 0).length || 0,
    claimed: tickets?.filter((t) => t.claimed).length || 0,
  };

  if (!address) {
    return (
      <div className="max-w-2xl mx-auto text-center py-16">
        <div className="w-20 h-20 bg-gray-800 rounded-full flex items-center justify-center mx-auto mb-6">
          <Wallet className="w-10 h-10 text-gray-400" />
        </div>
        <h1 className="text-3xl font-bold text-white mb-4">Connect Your Wallet</h1>
        <p className="text-gray-400 mb-8">
          Connect your Neo N3 wallet to view your lottery tickets.
        </p>
      </div>
    );
  }

  return (
    <div className="max-w-4xl mx-auto">
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1 className="text-3xl font-bold text-white mb-2">My Tickets</h1>
          <p className="text-gray-400">View and manage your lottery tickets</p>
        </div>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-8">
        <div className="glass rounded-xl p-4 text-center">
          <div className="text-2xl font-bold text-white">{stats.total}</div>
          <div className="text-sm text-gray-400">Total Tickets</div>
        </div>
        <div className="glass rounded-xl p-4 text-center">
          <div className="text-2xl font-bold text-yellow-400">{stats.pending}</div>
          <div className="text-sm text-gray-400">Pending</div>
        </div>
        <div className="glass rounded-xl p-4 text-center">
          <div className="text-2xl font-bold text-green-400">{stats.won}</div>
          <div className="text-sm text-gray-400">Won</div>
        </div>
        <div className="glass rounded-xl p-4 text-center">
          <div className="text-2xl font-bold text-purple-400">{stats.claimed}</div>
          <div className="text-sm text-gray-400">Claimed</div>
        </div>
      </div>

      {/* Filters */}
      <div className="flex items-center gap-2 mb-6">
        <Filter className="w-4 h-4 text-gray-400" />
        <span className="text-gray-400 text-sm">Filter:</span>
        {(['all', 'pending', 'won', 'lost'] as FilterType[]).map((f) => (
          <button
            key={f}
            onClick={() => setFilter(f)}
            className={`px-3 py-1 rounded-lg text-sm font-medium transition-colors ${
              filter === f
                ? 'bg-yellow-500 text-black'
                : 'bg-gray-800 text-gray-400 hover:text-white'
            }`}
          >
            {f.charAt(0).toUpperCase() + f.slice(1)}
          </button>
        ))}
      </div>

      {/* Tickets List */}
      {isLoading ? (
        <div className="text-center py-12">
          <div className="w-12 h-12 border-4 border-yellow-500 border-t-transparent rounded-full animate-spin mx-auto mb-4" />
          <p className="text-gray-400">Loading tickets...</p>
        </div>
      ) : filteredTickets && filteredTickets.length > 0 ? (
        <div className="grid md:grid-cols-2 gap-4">
          {filteredTickets.map((ticket) => {
            const draw = drawMap.get(ticket.drawId);
            return (
              <TicketCard
                key={ticket.ticketId}
                ticket={ticket}
                winningNumbers={draw?.winningNumbers}
                winningMega={draw?.winningMega}
                drawCompleted={draw?.completed}
              />
            );
          })}
        </div>
      ) : (
        <div className="text-center py-12">
          <div className="w-20 h-20 bg-gray-800 rounded-full flex items-center justify-center mx-auto mb-6">
            <Ticket className="w-10 h-10 text-gray-400" />
          </div>
          <h2 className="text-xl font-semibold text-white mb-2">No Tickets Found</h2>
          <p className="text-gray-400 mb-6">
            {filter === 'all'
              ? "You haven't purchased any tickets yet."
              : `No ${filter} tickets found.`}
          </p>
          {filter === 'all' && (
            <a
              href="/buy"
              className="inline-flex items-center gap-2 bg-yellow-500 hover:bg-yellow-400 text-black font-medium px-6 py-3 rounded-xl transition-colors"
            >
              <Ticket className="w-5 h-5" />
              Buy Your First Ticket
            </a>
          )}
        </div>
      )}
    </div>
  );
}
